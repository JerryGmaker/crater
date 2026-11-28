package image

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/raids-lab/crater/pkg/crclient"
)

func TestGetBuildPodNameReturnsEmptyWhenNameMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	mgr := &ImagePackMgr{}
	name, namespace, node := mgr.getBuildPodName(ctx, "")
	if name != "" || namespace != UserNameSpace || node != "" {
		t.Fatalf("result = (%q, %q, %q), want empty pod fields and namespace %q", name, namespace, node, UserNameSpace)
	}
}

func TestGetBuildPodNameReturnsEmptyWhenPodMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	mgr := &ImagePackMgr{imagepackClient: &crclient.ImagePackController{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}}
	name, namespace, node := mgr.getBuildPodName(ctx, "missing-image-pack")
	if name != "" || namespace != UserNameSpace || node != "" {
		t.Fatalf("result = (%q, %q, %q), want empty pod fields and namespace %q", name, namespace, node, UserNameSpace)
	}
}

func TestGetBuildPodNameReturnsPodMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	pod := &corev1.Pod{}
	pod.Name = "image-pack-pod"
	pod.Namespace = UserNameSpace
	pod.Labels = map[string]string{"job-name": "image-pack"}
	pod.Spec.NodeName = "node-a"
	mgr := &ImagePackMgr{imagepackClient: &crclient.ImagePackController{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build(),
	}}
	name, namespace, node := mgr.getBuildPodName(ctx, "image-pack")
	if name != pod.Name || namespace != UserNameSpace || node != pod.Spec.NodeName {
		t.Fatalf("result = (%q, %q, %q), want (%q, %q, %q)", name, namespace, node, pod.Name, UserNameSpace, pod.Spec.NodeName)
	}
}
