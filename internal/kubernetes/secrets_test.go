package kubernetes

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFetchSecretValue(t *testing.T) {
	// Create a fake client
	clientset := fake.NewSimpleClientset()

	// Create a secret
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"test-key": []byte("test-value"),
		},
	}

	// Add the secret to the fake client
	clientset.CoreV1().Secrets("default").Create(context.TODO(), secret, metav1.CreateOptions{})

	// Create a K8sClient with the fake client
	sm := &K8sClient{clientset: clientset}

	// Fetch the secret
	value, err := sm.FetchSecretValue("default", "test-secret", "test-key")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedValue := "test-value"
	if value != expectedValue {
		t.Fatalf("expected %s, got %s", expectedValue, value)
	}
}

func TestUpdateSecretValue(t *testing.T) {
	// Create a fake client
	clientset := fake.NewSimpleClientset()

	// Create a secret
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"test-key": []byte("test-value"),
		},
	}

	// Add the secret to the fake client
	clientset.CoreV1().Secrets("default").Create(context.TODO(), secret, metav1.CreateOptions{})

	// Create a K8sClient with the fake client
	sm := &K8sClient{clientset: clientset}

	// Update the secret
	newYamlValue := "new-test-value"

	err := sm.UpdateSecretValue("default", "test-secret", "test-key", newYamlValue, false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Fetch the updated secret
	updatedSecret, err := clientset.CoreV1().Secrets("default").Get(context.TODO(), "test-secret", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	decodedValue, ok := updatedSecret.Data["test-key"]
	if !ok {
		t.Fatalf("expected key test-key to be present")
	}

	if string(decodedValue) != newYamlValue {
		t.Fatalf("expected %s, got %s", newYamlValue, string(decodedValue))
	}
}
