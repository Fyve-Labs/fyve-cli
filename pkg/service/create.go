package service

import (
	"context"
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/config"
	"io"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kconfig "knative.dev/client/pkg/config"
	clientservingv1 "knative.dev/client/pkg/serving/v1"
	"knative.dev/client/pkg/wait"
	"knative.dev/serving/pkg/apis/autoscaling"
	"knative.dev/serving/pkg/apis/serving"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"time"
)

func CreateService(ctx context.Context, client clientservingv1.KnServingClient, namespace string, appConfig *config.AppConfig, env map[string]string, forceCreate bool, out io.Writer) error {
	service := &servingv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appConfig.App,
			Namespace: namespace,
		},
	}

	service.Spec.Template = servingv1.RevisionTemplateSpec{
		Spec: servingv1.RevisionSpec{},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				autoscaling.ScaleDownDelayAnnotationKey: appConfig.Autoscaling.ScaledownDelay,
			},
		},
	}

	service.Spec.Template.Spec.Containers = []corev1.Container{{
		Image: appConfig.Image,
		Env:   envMapToEnvvar(env),
		Ports: []corev1.ContainerPort{{
			ContainerPort: appConfig.Port,
			Protocol:      corev1.ProtocolTCP,
		}},
	}}

	serviceExists, err := serviceExists(ctx, client, service.Name)
	if err != nil {
		return err
	}

	if serviceExists {
		if !forceCreate {
			return fmt.Errorf(
				"cannot create service '%s' in namespace '%s' "+
					"because the service already exists and no --force option was given", service.Name, namespace)
		}
		err = replaceService(ctx, client, service, out)
	} else {
		err = createService(ctx, client, service, out)
	}

	return err
}

func envMapToEnvvar(env map[string]string) []corev1.EnvVar {
	envVars := make([]corev1.EnvVar, 0)
	for k, v := range env {
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	return envVars
}

func serviceExists(ctx context.Context, client clientservingv1.KnServingClient, name string) (bool, error) {
	_, err := client.GetService(ctx, name)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func createService(ctx context.Context, client clientservingv1.KnServingClient, service *servingv1.Service, out io.Writer) error {
	err := client.CreateService(ctx, service)
	if err != nil {
		return err
	}

	return waitIfRequested(ctx, client, service.Name, "Creating", "created", out)
}

func replaceService(ctx context.Context, client clientservingv1.KnServingClient, service *servingv1.Service, out io.Writer) error {
	changed, err := prepareAndUpdateService(ctx, client, service)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintf(out, "Service '%s' replaced in namespace '%s' (unchanged).\n", service.Name, client.Namespace())
		return nil
	}
	return waitIfRequested(ctx, client, service.Name, "Replacing", "replaced", out)
}

func prepareAndUpdateService(ctx context.Context, client clientservingv1.KnServingClient, service *servingv1.Service) (bool, error) {
	updateFunc := func(origService *servingv1.Service) (*servingv1.Service, error) {

		// Copy over some annotations that we want to keep around. Erase others
		copyList := []string{
			serving.CreatorAnnotation,
			serving.UpdaterAnnotation,
		}

		// If the target Annotation doesn't exist, create it even if
		// we don't end up copying anything over so that we erase all
		// existing annotations
		if service.Annotations == nil {
			service.Annotations = map[string]string{}
		}

		// Do the actual copy now, but only if it's in the source annotation
		for _, k := range copyList {
			if v, ok := origService.Annotations[k]; ok {
				service.Annotations[k] = v
			}
		}

		service.ResourceVersion = origService.ResourceVersion
		return service, nil
	}
	return client.UpdateServiceWithRetry(ctx, service.Name, updateFunc, kconfig.DefaultRetry.Steps)

}

func waitIfRequested(ctx context.Context, client clientservingv1.KnServingClient, serviceName string, verbDoing string, verbDone string, out io.Writer) error {
	fmt.Fprintf(out, "%s service '%s' in namespace '%s':\n", verbDoing, serviceName, client.Namespace())
	wconfig := clientservingv1.WaitConfig{
		Timeout:     time.Duration(600) * time.Second,
		ErrorWindow: time.Duration(2) * time.Second,
	}

	return waitForServiceToGetReady(ctx, client, serviceName, wconfig, verbDone, out)
}

func waitForServiceToGetReady(ctx context.Context, client clientservingv1.KnServingClient, name string, wconfig clientservingv1.WaitConfig, verbDone string, out io.Writer) error {
	fmt.Fprintln(out, "")
	err := waitForService(ctx, client, name, out, wconfig)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "")
	return showUrl(ctx, client, name, "", verbDone, out)
}

func waitForService(ctx context.Context, client clientservingv1.KnServingClient, serviceName string, out io.Writer, wconfig clientservingv1.WaitConfig) error {
	err, duration := client.WaitForService(ctx, serviceName, wconfig, wait.SimpleMessageCallback(out))
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%7.3fs Ready to serve.\n", float64(duration.Round(time.Millisecond))/float64(time.Second))
	return nil
}

func showUrl(ctx context.Context, client clientservingv1.KnServingClient, serviceName string, originalRevision string, what string, out io.Writer) error {
	service, err := client.GetService(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("cannot fetch service '%s' in namespace '%s' for extracting the URL: %w", serviceName, client.Namespace(), err)
	}

	url := service.Status.URL.String()

	newRevision := service.Status.LatestReadyRevisionName
	if (originalRevision != "" && originalRevision == newRevision) || originalRevision == "unchanged" {
		fmt.Fprintf(out, "Service '%s' with latest revision '%s' (unchanged) is available at URL:\n%s\n", serviceName, newRevision, url)
	} else {
		fmt.Fprintf(out, "Service '%s' %s to latest revision '%s' is available at URL:\n%s\n", serviceName, what, newRevision, url)
	}

	return nil
}
