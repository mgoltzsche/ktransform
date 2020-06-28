package e2e

import (
	"context"
	"testing"
	"time"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	dynclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func WaitForCondition(t *testing.T, obj runtime.Object, pollTimeout time.Duration, condition func() []string) (err error) {
	key, err := dynclient.ObjectKeyFromObject(obj)
	if err != nil {
		return
	}
	t.Logf("waiting up to %v for %s condition...", pollTimeout, key.Name)
	err = wait.PollImmediate(time.Second, pollTimeout, func() (bool, error) {
		if e := framework.Global.Client.Get(context.TODO(), key, obj); e != nil {
			t.Logf("%s not found: %s", key, e)
			return false, nil
		}
		if c := condition(); len(c) > 0 {
			t.Logf("  %s did not meet condition: %v", key, c)
			return false, nil
		}
		t.Logf("%s met condition", key)
		return true, nil
	})
	return
}
