package controller

import (
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ReconcilliationFinished bool = true
const ReconcilliationIncomplete bool = false

type Resource interface {
	*platformv1.JobRequest | *platformv1.JobRequestReview
	GetName() string
	GetNamespace() string
}

type Reconciler[R Resource] struct {
	CacheClient     client.Client
	ApiServerClient client.Reader
	Scheme          *runtime.Scheme
	Recorder        events.EventRecorder
	ResourceTtl     time.Duration
	Log             logr.Logger
	ReconcilerName  string
}

func (r *Reconciler[R]) LogReconcillationExit(complete bool, message string, resource R, err error, kvArgs ...any) {
	var exitMessage = message
	if complete {
		exitMessage += " Ending reconciliation."
	} else {
		exitMessage += " Reconcile will try again."
	}

	if err != nil {
		r.LogError(err, exitMessage, resource, kvArgs...)
	} else {
		r.LogInfo(exitMessage, resource, kvArgs...)
	}
}

func (r *Reconciler[R]) LogError(err error, message string, resource R, kvArgs ...any) {
	r.Log.Error(
		err,
		fmt.Sprintf("[%s] %s", r.ReconcilerName, message),
		r.logMessageArgs(resource, kvArgs)...,
	)
}

func (r *Reconciler[R]) LogInfo(message string, resource R, kvArgs ...any) {
	r.Log.Info(
		fmt.Sprintf("[%s] %s", r.ReconcilerName, message),
		r.logMessageArgs(resource, kvArgs)...,
	)
}

func (r *Reconciler[R]) logMessageArgs(resource R, kvArgs []any) []any {
	resourceName := reflect.TypeOf(resource).Elem().Name() + "Name"

	defaultArgs := []any{
		resourceName, resource.GetName(),
		"Namespace", resource.GetNamespace(),
	}

	return append(defaultArgs, kvArgs...)
}
