package secrettransform

import (
	"context"
	"crypto/sha256"
	goerrors "errors"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	ktransformv1alpha1 "github.com/mgoltzsche/ktransform/pkg/apis/ktransform/v1alpha1"
	"github.com/mgoltzsche/ktransform/pkg/backrefs"
	"github.com/mgoltzsche/ktransform/pkg/transform"
	"github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/util/hash"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log                      = logf.Log.WithName("controller_secrettransform")
	errAmbiguousResource     = goerrors.New("configMap or secret required but both specified")
	errUnspecifiedResource   = goerrors.New("neither configMap or secret specified")
	errMissingTransformation = goerrors.New("no transformation specified")
	finalizer                = "ktransform.mgoltzsche.github.com/finalizer"
)

const (
	jqQueryTimeout = time.Second * 5
)

func isSpecError(err error) bool {
	u := goerrors.Unwrap(err)
	return u == errAmbiguousResource || u == errUnspecifiedResource || u == errMissingTransformation
}

// Add creates a new SecretTransform Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	refHandler := backrefs.NewBackReferencesHandler(mgr.GetClient(), backrefs.OwnerReferences())
	r := &ReconcileSecretTransform{
		client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		restMapper: mgr.GetRESTMapper(),
		refhandler: refHandler}

	// Create a new controller
	c, err := controller.New("secrettransform-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource SecretTransform
	err = c.Watch(&source.Kind{Type: &ktransformv1alpha1.SecretTransform{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resources
	for _, res := range []runtime.Object{&corev1.Secret{}, &corev1.ConfigMap{}} {
		err = c.Watch(&source.Kind{Type: res}, &handler.EnqueueRequestForOwner{
			IsController: false,
			OwnerType:    &ktransformv1alpha1.SecretTransform{},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// blank assignment to verify that ReconcileSecretTransform implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSecretTransform{}

// ReconcileSecretTransform reconciles a SecretTransform object
type ReconcileSecretTransform struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	restMapper meta.RESTMapper
	refhandler *backrefs.BackReferencesHandler
}

// Reconcile reads that state of the cluster for a SecretTransform object and makes changes based on the state read
// and what is in the SecretTransform.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSecretTransform) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SecretTransform")

	// Fetch the SecretTransform instance
	cr := &ktransformv1alpha1.SecretTransform{}
	err := r.client.Get(context.TODO(), request.NamespacedName, cr)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	refOwner := &referenceOwner{cr}

	// When marked as deleted finalize object: remove back references
	if !cr.ObjectMeta.DeletionTimestamp.IsZero() {
		if hasFinalizer(cr, finalizer) {
			err = r.refhandler.UpdateReferences(context.TODO(), refOwner, nil)
			if err != nil {
				reqLogger.Error(err, "finalizer %s failed to clean up ownerReferences", finalizer)
				return reconcile.Result{}, err
			}
			controllerutil.RemoveFinalizer(cr, finalizer)
			err = r.client.Update(context.TODO(), cr)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// Fetch inputs
	refs, scope, err := r.inputScopeFactory(cr.Namespace, cr.Spec.Input)
	if err != nil {
		if errors.IsNotFound(goerrors.Unwrap(err)) {
			err = r.setSyncStatus(cr, corev1.ConditionFalse, ktransformv1alpha1.ReasonMissingInput, err.Error())
			return reconcile.Result{RequeueAfter: 30 * time.Second}, err
		}
		if isSpecError(err) {
			err = r.setSyncStatus(cr, corev1.ConditionFalse, ktransformv1alpha1.ReasonInvalidSpec, err.Error())
			return reconcile.Result{}, err
		}
		r.setSyncStatus(cr, corev1.ConditionFalse, ktransformv1alpha1.ReasonFailed, err.Error())
		return reconcile.Result{}, err
	}

	// Add CR as ownerReference to referenced Secrets/ConfigMaps
	controllerutil.AddFinalizer(cr, finalizer)
	err = r.refhandler.UpdateReferences(context.TODO(), refOwner, refs)
	if err != nil {
		r.setSyncStatus(cr, corev1.ConditionFalse, ktransformv1alpha1.ReasonFailed, err.Error())
		return reconcile.Result{}, err
	}

	// Transform
	transformed, err := transformedResources(scope, cr.Spec.Output)
	if err != nil {
		err = r.setSyncStatus(cr, corev1.ConditionFalse, ktransformv1alpha1.ReasonInvalidSpec, err.Error())
		return reconcile.Result{}, err // do not reconcile unless spec (or referenced resource) changes
	}
	applied := make([]runtime.Object, len(transformed))
	for i, tr := range transformed {
		applied[i] = tr.Resource
	}

	// Write output
	for _, res := range transformed {
		res.Resource.SetNamespace(cr.Namespace)
		var opRes controllerutil.OperationResult
		opRes, err = controllerutil.CreateOrUpdate(context.TODO(), r.client, res.Resource, func() error {
			res.Apply()
			return controllerutil.SetControllerReference(cr, res.Resource, r.scheme)
		})
		if err != nil {
			r.setSyncStatus(cr, corev1.ConditionFalse, ktransformv1alpha1.ReasonFailedWrite, err.Error())
			return reconcile.Result{}, err
		}
		switch opRes {
		case controllerutil.OperationResultCreated:
			logOperation(reqLogger, "Created output", res.Resource)
		case controllerutil.OperationResultUpdated:
			logOperation(reqLogger, "Updated output", res.Resource)
		}
	}

	// Update status
	h := sha256.New()
	hash.DeepHashObject(h, applied)
	outputHash := fmt.Sprintf("%x", h.Sum(nil))
	syncCond := status.Condition{
		Type:   ktransformv1alpha1.ConditionSynced,
		Status: corev1.ConditionTrue,
	}
	if cr.Status.Conditions.SetCondition(syncCond) ||
		cr.Status.OutputHash != outputHash ||
		cr.Status.ObservedGeneration != cr.Generation {
		cr.Status.ObservedGeneration = cr.Generation
		cr.Status.OutputHash = outputHash
		err = r.client.Status().Update(context.TODO(), cr)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func logOperation(log logr.Logger, verb string, o metav1.Object) {
	kind := reflect.TypeOf(o).Elem().Name()
	msg := fmt.Sprintf("%s %s", verb, kind)
	log.Info(msg, kind+".Namespace", o.GetNamespace(), kind+".Name", o.GetName())
}

func hasFinalizer(cr *ktransformv1alpha1.SecretTransform, final string) bool {
	for _, f := range cr.ObjectMeta.Finalizers {
		if f == final {
			return true
		}
	}
	return false
}

type resource interface {
	metav1.Object
	runtime.Object
}

type transformedResource struct {
	Resource resource
	Apply    func()
}

func transformedResources(inputs func() map[string]interface{}, outputs []ktransformv1alpha1.Output) ([]*transformedResource, error) {
	result := make([]*transformedResource, len(outputs))
	for i, out := range outputs {
		transformed, err := transformResource(inputs(), out)
		if err != nil {
			return nil, fmt.Errorf("output %d: %w", i, err)
		}
		result[i] = transformed
	}
	return result, nil
}

func transformResource(inputs map[string]interface{}, out ktransformv1alpha1.Output) (*transformedResource, error) {
	if len(out.Transformation) == 0 {
		return nil, errMissingTransformation
	}
	configMapName := ""
	if out.ConfigMap != nil {
		configMapName = out.ConfigMap.Name
	}
	secretName := ""
	if out.Secret != nil {
		secretName = out.Secret.Name
	}
	if configMapName != "" && secretName != "" {
		return nil, errAmbiguousResource
	}
	if configMapName == "" && secretName == "" {
		return nil, errUnspecifiedResource
	}
	ctx := context.TODO()
	transformed := map[string]interface{}{}
	for k, query := range out.Transformation {
		ctx, _ = context.WithTimeout(ctx, jqQueryTimeout)
		v, err := transform.Query(ctx, inputs, query)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		transformed[k] = v
	}
	if configMapName != "" {
		m, err := transform.StringMapFromOutput(transformed)
		if err != nil {
			return nil, err
		}
		cm := &corev1.ConfigMap{}
		cm.Name = configMapName
		return &transformedResource{cm, func() { cm.Data = m }}, nil
	}
	m, err := transform.BytesMapFromOutput(transformed)
	if err != nil {
		return nil, err
	}
	sec := &corev1.Secret{}
	sec.Name = secretName
	return &transformedResource{sec, func() { sec.Data = m }}, nil
}

func (r *ReconcileSecretTransform) setSyncStatus(cr *ktransformv1alpha1.SecretTransform, s corev1.ConditionStatus, reason status.ConditionReason, msg string) error {
	syncCond := status.Condition{
		Type:    ktransformv1alpha1.ConditionSynced,
		Status:  s,
		Reason:  reason,
		Message: msg,
	}
	if cr.Status.Conditions.SetCondition(syncCond) ||
		cr.Status.ObservedGeneration != cr.Generation {
		cr.Status.ObservedGeneration = cr.Generation
		return r.client.Status().Update(context.TODO(), cr)
	}
	return nil
}

func (r *ReconcileSecretTransform) inputScopeFactory(namespace string, inputs map[string]ktransformv1alpha1.InputRef) (l []backrefs.Object, inputFactory func() map[string]interface{}, err error) {
	constr := map[string]func() interface{}{}
	for k, v := range inputs {
		res, fn, err := r.loadInput(namespace, v)
		if err != nil {
			return nil, nil, fmt.Errorf("input %s: %w", k, err)
		}
		l = append(l, res)
		constr[k] = fn
	}
	return l, func() map[string]interface{} {
		scope := map[string]interface{}{}
		for k, v := range constr {
			scope[k] = v()
		}
		return scope
	}, nil
}

func (r *ReconcileSecretTransform) loadInput(namespace string, input ktransformv1alpha1.InputRef) (backrefs.Object, func() interface{}, error) {
	configMapName := ""
	if input.ConfigMap != nil {
		configMapName = *input.ConfigMap
	}
	secretName := ""
	if input.Secret != nil {
		secretName = *input.Secret
	}
	if configMapName != "" && secretName != "" {
		return nil, nil, errAmbiguousResource
	}
	if configMapName == "" && secretName == "" {
		return nil, nil, errUnspecifiedResource
	}
	if configMapName != "" {
		key := types.NamespacedName{Name: configMapName, Namespace: namespace}
		cm := &corev1.ConfigMap{}
		err := r.client.Get(context.TODO(), key, cm)
		return cm, func() interface{} {
			return transform.InputMapFromStringMap(cm.Data)
		}, err
	}
	key := types.NamespacedName{Name: secretName, Namespace: namespace}
	sec := &corev1.Secret{}
	err := r.client.Get(context.TODO(), key, sec)
	return sec, func() interface{} {
		return transform.InputMapFromBytesMap(sec.Data)
	}, err
}
