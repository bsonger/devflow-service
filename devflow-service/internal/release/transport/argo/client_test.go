package argoclient

import (
	"context"
	"testing"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type stubApplications struct {
	items map[string]*appv1.Application
}

func (s *stubApplications) Create(_ context.Context, app *appv1.Application, _ metav1.CreateOptions) (*appv1.Application, error) {
	if s.items == nil {
		s.items = map[string]*appv1.Application{}
	}
	s.items[app.Name] = app
	return app, nil
}

func (s *stubApplications) Get(_ context.Context, name string, _ metav1.GetOptions) (*appv1.Application, error) {
	if app, ok := s.items[name]; ok {
		return app, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, name)
}

func (s *stubApplications) Update(_ context.Context, app *appv1.Application, _ metav1.UpdateOptions) (*appv1.Application, error) {
	if s.items == nil {
		s.items = map[string]*appv1.Application{}
	}
	s.items[app.Name] = app
	return app, nil
}

func TestApplyApplicationCreatesWhenMissing(t *testing.T) {
	apps := &stubApplications{}
	app := &appv1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-app"},
	}

	if err := applyApplication(context.Background(), apps, app); err != nil {
		t.Fatalf("applyApplication returned error: %v", err)
	}
	if _, ok := apps.items["demo-app"]; !ok {
		t.Fatal("expected application to be created")
	}
}
