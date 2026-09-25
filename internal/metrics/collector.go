package metrics

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
)

type MetricCollector struct {
	CacheClient   client.Client
	Interval      time.Duration
	Log           logr.Logger
	CustomMetrics platformv1.CollectorCustomMetrics
}

type Resource interface {
	platformv1.JobRequest | platformv1.JobRequestReview
}

func (c *MetricCollector) Start(ctx context.Context) error {
	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

func (c *MetricCollector) collect(ctx context.Context) {
	requestList, err := c.listRequests(ctx)
	if err != nil {
		c.Log.Error(err, "[CollectorCustomMetrics] Failed to collect metrics")
		return
	}

	reviewList, err := c.listReviews(ctx)
	if err != nil {
		c.Log.Error(err, "[CollectorCustomMetrics] Failed to collect metrics")
		return
	}

	c.handleMetrics(requestList)
	c.handleMetrics(reviewList)
}

func (c *MetricCollector) handleMetrics(list client.ObjectList) {
	switch l := list.(type) {
	case *platformv1.JobRequestList:
		c.emitRequestMetrics(tallyResourceState(getRequestState)(initRequestStateMap(), l.Items))
	case *platformv1.JobRequestReviewList:
		c.emitReviewMetrics(tallyResourceState(getReviewState)(initReviewStateMap(), l.Items))
	}
}

func (c *MetricCollector) listRequests(ctx context.Context) (*platformv1.JobRequestList, error) {
	jobRequestList := platformv1.JobRequestList{}
	err := c.CacheClient.List(ctx, &jobRequestList)
	if err != nil {
		return nil, errors.New("[CollectorCustomMetrics] Failed to list Job Requests from the cache")
	}

	return &jobRequestList, nil
}

func (c *MetricCollector) emitRequestMetrics(requestStateTotals map[platformv1.JobRequestState]float64) {
	for k, v := range requestStateTotals {
		switch k {
		case platformv1.JobRequestPending:
			c.CustomMetrics.RequestCurrentPendingTotal.Set(v)
		case platformv1.JobRequestApproved:
			c.CustomMetrics.RequestCurrentApprovedTotal.Set(v)
		case platformv1.JobRequestRejected:
			c.CustomMetrics.RequestCurrentRejectedTotal.Set(v)
		case platformv1.JobRequestStarted:
			c.CustomMetrics.RequestCurrentStartedTotal.Set(v)
		case platformv1.JobRequestComplete:
			c.CustomMetrics.RequestCurrentCompleteTotal.Set(v)
		case platformv1.JobRequestFailed:
			c.CustomMetrics.RequestCurrentFailedTotal.Set(v)
		case platformv1.JobRequestMalformed:
			c.CustomMetrics.RequestCurrentMalformedTotal.Set(v)
		}
	}
}

func (c *MetricCollector) listReviews(ctx context.Context) (*platformv1.JobRequestReviewList, error) {
	jobRequestReviewList := platformv1.JobRequestReviewList{}
	err := c.CacheClient.List(ctx, &jobRequestReviewList)
	if err != nil {
		return nil, errors.New("[CollectorCustomMetrics] Failed to list Job Request Reviews from the cache")
	}

	return &jobRequestReviewList, nil
}

func (c *MetricCollector) emitReviewMetrics(reviewStateTotals map[platformv1.JobRequestReviewState]float64) {
	for k, v := range reviewStateTotals {
		switch k {
		case platformv1.JobRequestReviewApproved:
			c.CustomMetrics.ReviewCurrentApprovedTotal.Set(v)
		case platformv1.JobRequestReviewRejected:
			c.CustomMetrics.ReviewCurrentRejectedTotal.Set(v)
		case platformv1.JobRequestReviewMalformed:
			c.CustomMetrics.ReviewCurrentMalformedTotal.Set(v)
		case platformv1.JobRequestReviewNotFound:
			c.CustomMetrics.ReviewCurrentNotFoundTotal.Set(v)
		case platformv1.JobRequestReviewConflict:
			c.CustomMetrics.ReviewCurrentConflictTotal.Set(v)
		}
	}
}

func initRequestStateMap() map[platformv1.JobRequestState]float64 {
	return map[platformv1.JobRequestState]float64{
		platformv1.JobRequestPending:   0,
		platformv1.JobRequestApproved:  0,
		platformv1.JobRequestRejected:  0,
		platformv1.JobRequestStarted:   0,
		platformv1.JobRequestComplete:  0,
		platformv1.JobRequestFailed:    0,
		platformv1.JobRequestMalformed: 0,
	}
}

func initReviewStateMap() map[platformv1.JobRequestReviewState]float64 {
	return map[platformv1.JobRequestReviewState]float64{
		platformv1.JobRequestReviewApproved:  0,
		platformv1.JobRequestReviewRejected:  0,
		platformv1.JobRequestReviewMalformed: 0,
		platformv1.JobRequestReviewNotFound:  0,
		platformv1.JobRequestReviewConflict:  0,
	}
}

func getRequestState(r platformv1.JobRequest) platformv1.JobRequestState {
	return r.Status.State
}

func getReviewState(r platformv1.JobRequestReview) platformv1.JobRequestReviewState {
	return r.Status.State
}

func tallyResourceState[T Resource, K comparable](getState func(T) K) func(map[K]float64, []T) map[K]float64 {
	return func(resourceMap map[K]float64, resourceList []T) map[K]float64 {
		for _, v := range resourceList {
			state := getState(v)
			resourceMap[state]++
		}
		return resourceMap
	}
}
