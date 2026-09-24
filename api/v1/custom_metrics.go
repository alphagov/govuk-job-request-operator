package v1

import "github.com/prometheus/client_golang/prometheus"

// +kubebuilder:object:generate=false
type RequestCustomMetrics struct {
	ReceivedTotal                prometheus.Counter
	RequeueTotal                 *prometheus.CounterVec
	SuccessfulReconcileTotal     *prometheus.CounterVec
	ErrorGettingRequestTotal     *prometheus.CounterVec
	ErrorAlreadyDeletedTotal     *prometheus.CounterVec
	ErrorDeletingByTtlTotal      *prometheus.CounterVec
	DeletedByTtlTotal            *prometheus.CounterVec
	AlreadyInTerminalStateTotal  *prometheus.CounterVec
	ErrorRequestedByAnnoTotal    *prometheus.CounterVec
	NoneFoundTargetResourceTotal *prometheus.CounterVec
	ErrorCreateJobTotal          *prometheus.CounterVec
	PendingStateTotal            *prometheus.CounterVec
	ApprovedStateTotal           *prometheus.CounterVec
	RejectedStateTotal           *prometheus.CounterVec
	StartedStateTotal            *prometheus.CounterVec
	MalformedStateTotal          *prometheus.CounterVec
	JobCompleteStateTotal        *prometheus.CounterVec
	JobFailedStateTotal          *prometheus.CounterVec
	TimeTilReview                *prometheus.HistogramVec
	MetricLabels                 prometheus.Labels
}

// +kubebuilder:object:generate=false
type ReviewCustomMetrics struct {
	ReceivedTotal            prometheus.Counter
	RequeueTotal             *prometheus.CounterVec
	ErrorGettingReviewTotal  *prometheus.CounterVec
	ErrorAlreadyDeletedTotal *prometheus.CounterVec
	ErrorDeletingByTtlTotal  *prometheus.CounterVec
	DeletedByTtlTotal        *prometheus.CounterVec
	AlreadyHasStateTotal     *prometheus.CounterVec
	ErrorReviewByAnnoTotal   *prometheus.CounterVec
	ErrorGettingRequestTotal *prometheus.CounterVec
	NoRequestFoundTotal      *prometheus.CounterVec
	MalformedStateTotal      *prometheus.CounterVec
	NotFoundStateTotal       *prometheus.CounterVec
	ConflictStateTotal       *prometheus.CounterVec
	ApprovedStateTotal       *prometheus.CounterVec
	RejectedStateTotal       *prometheus.CounterVec
	SuccessfulReconcileTotal *prometheus.CounterVec
	MetricLabels             prometheus.Labels
}

// +kubebuilder:object:generate=false
type CollectorCustomMetrics struct {
	RequestCurrentPendingTotal   prometheus.Gauge
	RequestCurrentApprovedTotal  prometheus.Gauge
	RequestCurrentRejectedTotal  prometheus.Gauge
	RequestCurrentStartedTotal   prometheus.Gauge
	RequestCurrentCompleteTotal  prometheus.Gauge
	RequestCurrentFailedTotal    prometheus.Gauge
	RequestCurrentMalformedTotal prometheus.Gauge
	ReviewCurrentApprovedTotal   prometheus.Gauge
	ReviewCurrentRejectedTotal   prometheus.Gauge
	ReviewCurrentMalformedTotal  prometheus.Gauge
	ReviewCurrentNotFoundTotal   prometheus.Gauge
	ReviewCurrentConflictTotal   prometheus.Gauge
}
