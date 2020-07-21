package e2e

import (
	"testing"
	"time"

	"github.com/mgoltzsche/ktransform/pkg/apis"
	ktransformv1alpha1 "github.com/mgoltzsche/ktransform/pkg/apis/ktransform/v1alpha1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/stretchr/testify/require"
)

func TestOperator(t *testing.T) {
	err := framework.AddToFrameworkScheme(apis.AddToScheme, &ktransformv1alpha1.SecretTransform{})
	require.NoError(t, err)

	ctx := framework.NewContext(t)
	defer ctx.Cleanup()

	err = ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 30, RetryInterval: time.Second * 3})
	require.NoError(t, err)

	namespace, err := ctx.GetOperatorNamespace()
	require.NoError(t, err, "testctx.GetOperatorNamespace()")
	f := framework.Global
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "ktransform-operator", 1, time.Second*5, time.Second*30)
	require.NoError(t, err)

	t.Run("SecretTransform", func(t *testing.T) {
		testTransform(t, ctx)
	})
}
