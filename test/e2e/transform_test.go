package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	ktransformv1alpha1 "github.com/mgoltzsche/ktransform/pkg/apis/ktransform/v1alpha1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

func testTransform(t *testing.T, ctx *framework.Context) {
	f := framework.Global
	ns, err := ctx.GetNamespace()
	require.NoError(t, err)

	// Create resources
	usr := "user"
	pw := "passwd"
	prefix := "basic"
	cr := createTestData(t, prefix, ns, usr, pw)
	assertOutput(t, prefix, ns, usr, pw, "cmvalue", "registry0.example.org", "registry1.example.org")

	t.Run("input ConfigMap change should reconcile", func(t *testing.T) {
		prefix := "inputcmchange"
		cr := createTestData(t, prefix, ns, usr, pw)
		input := &corev1.ConfigMap{}
		inputKey := types.NamespacedName{Name: prefix + "-myconfig", Namespace: ns}
		err = f.Client.Get(context.Background(), inputKey, input)
		require.NoError(t, err)
		input.Data = map[string]string{"someprop": "changedCMValue"}
		err = f.Client.Update(context.Background(), input)
		require.NoError(t, err)
		waitForTransformation(t, cr, cr.Status.OutputHash, 10*time.Second)
		assertOutput(t, prefix, ns, usr, pw, "changedCMValue", "registry0.example.org", "registry1.example.org")
	})

	t.Run("input Secret change should reconcile", func(t *testing.T) {
		prefix := "inputsecretchange"
		cr := createTestData(t, prefix, ns, usr, pw)
		input := &corev1.Secret{}
		inputKey := types.NamespacedName{Name: prefix + "-mysecret0", Namespace: ns}
		err = f.Client.Get(context.Background(), inputKey, input)
		require.NoError(t, err)
		auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", usr, pw)))
		input.Data = map[string][]byte{corev1.DockerConfigJsonKey: []byte(fmt.Sprintf(`{"auths": {"changedregistry.example.org": {"auth": %q}}}`, auth))}
		err = f.Client.Update(context.Background(), input)
		require.NoError(t, err)
		waitForTransformation(t, cr, cr.Status.OutputHash, 10*time.Second)
		assertOutput(t, prefix, ns, usr, pw, "cmvalue", "changedregistry.example.org", "registry1.example.org")
	})

	t.Run("input ConfigMap deletion and recreation should reconcile", func(t *testing.T) {
		input := &corev1.ConfigMap{}
		inputKey := types.NamespacedName{Name: prefix + "-myconfig", Namespace: ns}
		err = f.Client.Get(context.Background(), inputKey, input)
		require.NoError(t, err)
		f.Client.Delete(context.Background(), input)
		waitForDesyncStatus(t, cr)
		require.NoError(t, err)
		input = &corev1.ConfigMap{}
		input.Name = prefix + "-myconfig"
		input.Namespace = ns
		input.Data = map[string]string{"someprop": "changedCMValue"}
		err := f.Client.Create(context.Background(), input, nil)
		require.NoError(t, err, "recreate input configmap")
		waitForTransformation(t, cr, cr.Status.OutputHash, 40*time.Second)
		assertOutput(t, prefix, ns, usr, pw, "changedCMValue", "registry0.example.org", "registry1.example.org")
	})
}

func createTestData(t *testing.T, prefix, ns, usr, pw string) *ktransformv1alpha1.SecretTransform {
	f := framework.Global
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", usr, pw)))
	secrets := []*corev1.Secret{}
	for i := 0; i < 2; i++ {
		sec := &corev1.Secret{}
		sec.Name = fmt.Sprintf("%s-mysecret%d", prefix, i)
		sec.Namespace = ns
		sec.Type = corev1.SecretTypeDockerConfigJson
		sec.Data = map[string][]byte{corev1.DockerConfigJsonKey: []byte(fmt.Sprintf(`{"auths": {"registry%d.example.org": {"auth": %q}}}`, i, auth))}
		err := f.Client.Create(context.Background(), sec, nil)
		require.NoError(t, err, "create input secret")
		secrets = append(secrets, sec)
	}
	cm := &corev1.ConfigMap{}
	cm.Name = prefix + "-myconfig"
	cm.Namespace = ns
	cm.Data = map[string]string{"someprop": "cmvalue"}
	err := f.Client.Create(context.Background(), cm, nil)
	require.NoError(t, err, "create input configmap")
	cr := &ktransformv1alpha1.SecretTransform{}
	cr.Name = prefix + "-mytransform"
	cr.Namespace = ns
	cr.Spec.Input = map[string]ktransformv1alpha1.InputRef{
		"secret0": ktransformv1alpha1.InputRef{Secret: &secrets[0].Name},
		"secret1": ktransformv1alpha1.InputRef{Secret: &secrets[1].Name},
		"config":  ktransformv1alpha1.InputRef{ConfigMap: &cm.Name},
	}
	cr.Spec.Output = []ktransformv1alpha1.Output{
		{
			Secret: &ktransformv1alpha1.SecretOutput{
				Name: prefix + "-mergedsecret",
				Type: corev1.SecretTypeOpaque,
			},
			Transformation: map[string]string{
				"makisu.conf": `(.secret0[".dockerconfigjson"].object.auths * .secret1[".dockerconfigjson"].object.auths) | with_entries(.value |= {".*": {security: {basic: .auth | @base64d | split(":") | {username: .[0], password: .[1]}}}})`,
				"someconf":    `.config.someprop.string`,
			},
		},
		{
			ConfigMap: &ktransformv1alpha1.ConfigMapOutput{
				Name: prefix + "-mergedconfigmap",
			},
			Transformation: map[string]string{
				"myconf": `{confKey: .config.someprop.string}`,
			},
		},
	}
	err = f.Client.Create(context.Background(), cr, nil)
	require.NoError(t, err, "create %T", cr)
	waitForTransformation(t, cr, "", 10*time.Second)
	return cr
}

func assertOutput(t *testing.T, prefix, ns, usr, pw, configMapValue string, registries ...string) {
	f := framework.Global
	// Assert transformed Secret
	outSecret := &corev1.Secret{}
	outKey := types.NamespacedName{Name: prefix + "-mergedsecret", Namespace: ns}
	err := f.Client.Get(context.Background(), outKey, outSecret)
	require.NoError(t, err, "get output secret")
	require.NotNil(t, outSecret.Data, "output secret data")
	require.Equal(t, configMapValue, string(outSecret.Data["someconf"]), "outSecret.data.someconf")
	makisuRegAuth := map[string]interface{}{".*": map[string]interface{}{"security": map[string]interface{}{"basic": map[string]interface{}{"username": usr, "password": pw}}}}
	expectedMakisuConf := map[string]interface{}{}
	for _, reg := range registries {
		expectedMakisuConf[reg] = makisuRegAuth
	}
	actual := map[string]interface{}{}
	err = yaml.Unmarshal(outSecret.Data["makisu.conf"], &actual)
	require.NoError(t, err, "unmarshal output makisu.conf")
	require.Equal(t, expectedMakisuConf, actual)

	// Assert transformed ConfigMap
	outCm := &corev1.ConfigMap{}
	outKey = types.NamespacedName{Name: prefix + "-mergedconfigmap", Namespace: ns}
	err = f.Client.Get(context.Background(), outKey, outCm)
	require.NoError(t, err, "get output configmap")
	require.NotNil(t, outCm.Data, "output configmap data")
	require.Equal(t, fmt.Sprintf(`{"confKey":%q}`, configMapValue), string(outCm.Data["myconf"]), "outConfigMap.data.myconf")
}

func waitForDesyncStatus(t *testing.T, cr *ktransformv1alpha1.SecretTransform) {
	err := WaitForCondition(t, cr, 10*time.Second, func() (c []string) {
		if !cr.Status.Conditions.IsFalseFor(ktransformv1alpha1.ConditionSynced) {
			c = append(c, "desync")
		}
		return
	})
	require.NoError(t, err)
}

func waitForTransformation(t *testing.T, cr *ktransformv1alpha1.SecretTransform, lastHash string, pollTimeout time.Duration) {
	err := WaitForCondition(t, cr, pollTimeout, func() (c []string) {
		if cr.Generation != cr.Status.ObservedGeneration {
			c = append(c, "observedGeneration")
		}
		if !cr.Status.Conditions.IsTrueFor(ktransformv1alpha1.ConditionSynced) {
			cond := cr.Status.Conditions.GetCondition(ktransformv1alpha1.ConditionSynced)
			s := "synced"
			if cond != nil {
				s = fmt.Sprintf("%s: %s: %s", s, cond.Reason, cond.Message)
			}
			c = append(c, s)
		}
		if cr.Status.OutputHash == lastHash {
			c = append(c, "outputHash")
		}
		return
	})
	require.NoError(t, err)
}
