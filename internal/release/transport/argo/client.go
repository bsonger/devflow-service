package argoclient

import (
	"context"
	"fmt"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argoapi "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

var Client ArgoClientInterface

const namespace = "argocd"

type ArgoClientInterface interface {
	ArgoprojV1alpha1() argov1alpha1.ArgoprojV1alpha1Interface
}

type applicationAPI interface {
	Create(ctx context.Context, app *appv1.Application, opts metav1.CreateOptions) (*appv1.Application, error)
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*appv1.Application, error)
	Update(ctx context.Context, app *appv1.Application, opts metav1.UpdateOptions) (*appv1.Application, error)
}

func Init(config *rest.Config) error {
	var err error
	Client, err = argoapi.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create argo cd client: %w", err)
	}
	logger.Logger.Info("argo cd client initialized")
	return nil
}

func CreateApplication(ctx context.Context, app *appv1.Application) error {
	_, err := Client.ArgoprojV1alpha1().Applications(namespace).Create(ctx, app, metav1.CreateOptions{})
	return err
}

func UpdateApplication(ctx context.Context, app *appv1.Application) error {
	return applyApplication(ctx, Client.ArgoprojV1alpha1().Applications(namespace), app)
}

func applyApplication(ctx context.Context, applications applicationAPI, app *appv1.Application) error {
	current, err := applications.Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = applications.Create(ctx, app, metav1.CreateOptions{})
			return err
		}
		return err
	}

	current.Spec = app.Spec
	current.Annotations = app.Annotations
	current.Labels = app.Labels

	_, err = applications.Update(ctx, current, metav1.UpdateOptions{})
	return err
}

type appProjectAPI interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*appv1.AppProject, error)
	Update(ctx context.Context, project *appv1.AppProject, opts metav1.UpdateOptions) (*appv1.AppProject, error)
}

func GetAppProject(ctx context.Context, name string) (*appv1.AppProject, error) {
	return Client.ArgoprojV1alpha1().AppProjects(namespace).Get(ctx, name, metav1.GetOptions{})
}

func UpdateAppProject(ctx context.Context, project *appv1.AppProject) error {
	_, err := Client.ArgoprojV1alpha1().AppProjects(namespace).Update(ctx, project, metav1.UpdateOptions{})
	return err
}
