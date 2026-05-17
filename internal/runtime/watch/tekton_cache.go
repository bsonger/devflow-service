package watch

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	manifestIDLabel   = "devflow.manifest/id"
	pipelineTaskLabel = "tekton.dev/pipelineTask"
)

type PipelineRunSnapshot struct {
	ManifestID     string
	Name           string
	Namespace      string
	ControlPlaneID string
	Status         string
	Message        string
	StateKey       string
}

type TaskRunSnapshot struct {
	ManifestID string
	PipelineID string
	TaskName   string
	TaskRun    string
	Status     string
	Message    string
	Params     map[string]string
	Results    map[string]string
}

type TektonSnapshot struct {
	PipelineRuns []PipelineRunSnapshot
	TaskRuns     map[string][]TaskRunSnapshot
}

type TektonCache interface {
	ListManifestIDs(controlPlaneID string) []string
	GetManifestSnapshot(manifestID string) (*TektonSnapshot, bool)
}

type PipelineRunListFunc func(context.Context) ([]tknv1.PipelineRun, error)
type TaskRunListFunc func(context.Context, string, string) ([]tknv1.TaskRun, error)

type tektonCache struct {
	mu           sync.RWMutex
	pipelineRuns []PipelineRunSnapshot
	taskRuns     map[string][]TaskRunSnapshot
}

type PollingTektonCache struct {
	tektonCache
	pipelineRunList PipelineRunListFunc
	taskRunList     TaskRunListFunc
	pipelineName    string
}

func NewPollingTektonCache(pipelineRunList PipelineRunListFunc, taskRunList TaskRunListFunc, pipelineName string) *PollingTektonCache {
	return &PollingTektonCache{
		tektonCache: tektonCache{
			taskRuns: map[string][]TaskRunSnapshot{},
		},
		pipelineRunList: pipelineRunList,
		taskRunList:     taskRunList,
		pipelineName:    strings.TrimSpace(pipelineName),
	}
}

func (c *PollingTektonCache) Refresh(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if c.pipelineRunList == nil || c.taskRunList == nil {
		return errors.New("tekton polling cache is not configured")
	}

	pipelineRuns, err := c.pipelineRunList(ctx)
	if err != nil {
		return err
	}

	type pipelineRunEntry struct {
		manifestID string
		snapshot   PipelineRunSnapshot
	}

	entries := make([]pipelineRunEntry, 0, len(pipelineRuns))
	for i := range pipelineRuns {
		if c.pipelineMatches(&pipelineRuns[i]) == false {
			continue
		}
		snapshot, ok := snapshotPipelineRun(&pipelineRuns[i])
		if !ok {
			continue
		}
		entries = append(entries, pipelineRunEntry{
			manifestID: snapshot.ManifestID,
			snapshot:   snapshot,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].manifestID != entries[j].manifestID {
			return entries[i].manifestID < entries[j].manifestID
		}
		return moreRecentPipelineRun(entries[i].snapshot, entries[j].snapshot)
	})

	nextPipelineRuns := make([]PipelineRunSnapshot, 0, len(entries))
	nextTaskRuns := make(map[string][]TaskRunSnapshot, len(entries))
	for _, entry := range entries {
		nextPipelineRuns = append(nextPipelineRuns, entry.snapshot)
		if _, exists := nextTaskRuns[entry.manifestID]; exists {
			continue
		}
		taskRuns, err := c.taskRunList(ctx, entry.snapshot.Namespace, entry.snapshot.Name)
		if err != nil {
			return err
		}
		nextTaskRuns[entry.manifestID] = snapshotTaskRuns(entry.snapshot.ManifestID, entry.snapshot.Name, taskRuns)
	}

	c.mu.Lock()
	c.pipelineRuns = nextPipelineRuns
	c.taskRuns = nextTaskRuns
	c.mu.Unlock()
	return nil
}

func (c *tektonCache) ListManifestIDs(controlPlaneID string) []string {
	c.mu.RLock()
	pipelineRuns := append([]PipelineRunSnapshot(nil), c.pipelineRuns...)
	c.mu.RUnlock()

	controlPlaneID = strings.TrimSpace(controlPlaneID)
	seen := make(map[string]struct{}, len(pipelineRuns))
	out := make([]string, 0, len(pipelineRuns))
	for _, item := range pipelineRuns {
		manifestID := strings.TrimSpace(item.ManifestID)
		if manifestID == "" {
			continue
		}
		if controlPlaneID != "" && strings.TrimSpace(item.ControlPlaneID) != controlPlaneID {
			continue
		}
		if _, ok := seen[manifestID]; ok {
			continue
		}
		seen[manifestID] = struct{}{}
		out = append(out, manifestID)
	}
	sort.Strings(out)
	return out
}

func (c *PollingTektonCache) pipelineMatches(pr *tknv1.PipelineRun) bool {
	if c == nil {
		return false
	}
	filter := strings.TrimSpace(c.pipelineName)
	if filter == "" {
		return true
	}
	if pr == nil || pr.Spec.PipelineRef == nil {
		return false
	}
	return strings.TrimSpace(pr.Spec.PipelineRef.Name) == filter
}

func (c *tektonCache) GetManifestSnapshot(manifestID string) (*TektonSnapshot, bool) {
	manifestID = strings.TrimSpace(manifestID)
	if manifestID == "" {
		return nil, false
	}

	c.mu.RLock()
	pipelineRuns := make([]PipelineRunSnapshot, 0, 1)
	for _, item := range c.pipelineRuns {
		if strings.TrimSpace(item.ManifestID) != manifestID {
			continue
		}
		pipelineRuns = append(pipelineRuns, item)
	}
	if len(pipelineRuns) == 0 {
		c.mu.RUnlock()
		return nil, false
	}

	taskRuns := append([]TaskRunSnapshot(nil), c.taskRuns[manifestID]...)
	c.mu.RUnlock()
	return &TektonSnapshot{
		PipelineRuns: pipelineRuns,
		TaskRuns: map[string][]TaskRunSnapshot{
			manifestID: taskRuns,
		},
	}, true
}

func pipelineRunObjectKey(namespace, name string) string {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return ""
	}
	return namespace + "/" + name
}

func pipelineRunKey(pr *tknv1.PipelineRun) string {
	if pr == nil {
		return ""
	}
	return pipelineRunObjectKey(pr.Namespace, pr.Name)
}

func taskRunPipelineKey(tr *tknv1.TaskRun) string {
	if tr == nil {
		return ""
	}
	return pipelineRunObjectKey(tr.Namespace, taskRunPipelineName(tr))
}

func taskRunPipelineName(tr *tknv1.TaskRun) string {
	if tr == nil {
		return ""
	}
	return strings.TrimSpace(tr.Labels["tekton.dev/pipelineRun"])
}

func snapshotPipelineRun(pr *tknv1.PipelineRun) (PipelineRunSnapshot, bool) {
	if pr == nil {
		return PipelineRunSnapshot{}, false
	}
	manifestID := strings.TrimSpace(pr.Labels[manifestIDLabel])
	if manifestID == "" {
		return PipelineRunSnapshot{}, false
	}
	return PipelineRunSnapshot{
		ManifestID:     manifestID,
		Name:           strings.TrimSpace(pr.Name),
		Namespace:      strings.TrimSpace(pr.Namespace),
		ControlPlaneID: strings.TrimSpace(pr.Labels[releasedomain.ControlPlaneLabel]),
		Status:         string(mapPipelineRunStatus(pr)),
		Message:        pipelineMessage(pr),
		StateKey:       pipelineRunStateKey(pr),
	}, true
}

func cloneTaskRun(item *tknv1.TaskRun) *tknv1.TaskRun {
	if item == nil {
		return nil
	}
	return item.DeepCopy()
}

func clonePipelineRun(item *tknv1.PipelineRun) *tknv1.PipelineRun {
	if item == nil {
		return nil
	}
	return item.DeepCopy()
}

func snapshotTaskRuns(manifestID, pipelineID string, taskRuns []tknv1.TaskRun) []TaskRunSnapshot {
	if len(taskRuns) == 0 {
		return nil
	}
	out := make([]TaskRunSnapshot, 0, len(taskRuns))
	for i := range taskRuns {
		out = append(out, TaskRunSnapshot{
			ManifestID: manifestID,
			PipelineID: pipelineID,
			TaskName:   taskRunName(&taskRuns[i]),
			TaskRun:    strings.TrimSpace(taskRuns[i].Name),
			Status:     string(mapTaskRunStatus(&taskRuns[i])),
			Message:    taskRunMessage(&taskRuns[i]),
			Params:     taskRunParams(&taskRuns[i]),
			Results:    taskRunResults(&taskRuns[i]),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TaskName != out[j].TaskName {
			return out[i].TaskName < out[j].TaskName
		}
		return out[i].TaskRun < out[j].TaskRun
	})
	return out
}

func taskRunParams(tr *tknv1.TaskRun) map[string]string {
	if tr == nil || len(tr.Spec.Params) == 0 {
		return nil
	}
	out := make(map[string]string, len(tr.Spec.Params))
	for _, param := range tr.Spec.Params {
		if name := strings.TrimSpace(param.Name); name != "" {
			out[name] = strings.TrimSpace(param.Value.StringVal)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func taskRunResults(tr *tknv1.TaskRun) map[string]string {
	if tr == nil || len(tr.Status.Results) == 0 {
		return nil
	}
	out := make(map[string]string, len(tr.Status.Results))
	for _, result := range tr.Status.Results {
		if name := strings.TrimSpace(result.Name); name != "" {
			out[name] = strings.TrimSpace(result.Value.StringVal)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type ManifestResultSnapshot struct {
	CommitHash  string
	ImageRef    string
	ImageTag    string
	ImageDigest string
}

func BuildManifestResultSnapshot(taskRuns []TaskRunSnapshot) *ManifestResultSnapshot {
	var result ManifestResultSnapshot
	for _, taskRun := range taskRuns {
		if result.CommitHash == "" {
			result.CommitHash = strings.TrimSpace(taskRun.Results["commit"])
		}
		if result.ImageTag == "" {
			result.ImageTag = strings.TrimSpace(taskRun.Results["IMAGE_TAG"])
			if result.ImageTag == "" {
				result.ImageTag = strings.TrimSpace(taskRun.Params["IMAGE_TAG"])
			}
		}
		if result.ImageDigest == "" {
			result.ImageDigest = strings.TrimSpace(taskRun.Results["IMAGE_DIGEST"])
		}
		if result.ImageRef == "" {
			result.ImageRef = imageRefFromTaskSnapshot(taskRun, result.ImageTag, result.ImageDigest)
		}
	}
	if strings.TrimSpace(result.CommitHash) == "" &&
		strings.TrimSpace(result.ImageRef) == "" &&
		strings.TrimSpace(result.ImageTag) == "" &&
		strings.TrimSpace(result.ImageDigest) == "" {
		return nil
	}
	return &result
}

func imageRefFromTaskSnapshot(taskRun TaskRunSnapshot, imageTag, imageDigest string) string {
	repository := strings.TrimSpace(taskRun.Params["IMAGE_REPOSITORY"])
	if repository == "" {
		return ""
	}
	if imageDigest != "" {
		return repository + "@" + imageDigest
	}
	if imageTag != "" {
		return repository + ":" + imageTag
	}
	return ""
}

func taskRunSnapshotsForPipeline(manifestID string, pipelineRun *tknv1.PipelineRun, taskRuns []*tknv1.TaskRun) []TaskRunSnapshot {
	if pipelineRun == nil || len(taskRuns) == 0 {
		return nil
	}
	items := make([]tknv1.TaskRun, 0, len(taskRuns))
	for _, taskRun := range taskRuns {
		if taskRun == nil {
			continue
		}
		if taskRunPipelineKey(taskRun) != pipelineRunKey(pipelineRun) {
			continue
		}
		items = append(items, *taskRun.DeepCopy())
	}
	return snapshotTaskRuns(manifestID, strings.TrimSpace(pipelineRun.Name), items)
}

func moreRecentPipelineRun(left, right PipelineRunSnapshot) bool {
	leftTime := snapshotStateTime(left)
	rightTime := snapshotStateTime(right)
	if !leftTime.Equal(rightTime) {
		return leftTime.After(rightTime)
	}
	if left.StateKey != right.StateKey {
		return left.StateKey > right.StateKey
	}
	return left.Name < right.Name
}

func snapshotStateTime(snapshot PipelineRunSnapshot) time.Time {
	if stateKey := strings.TrimSpace(snapshot.StateKey); stateKey != "" {
		parts := strings.SplitN(stateKey, "|", 2)
		if len(parts) == 2 {
			if ts, err := time.Parse(time.RFC3339Nano, parts[1]); err == nil {
				return ts
			}
		}
	}
	return time.Time{}
}

func mapPipelineRunStatus(pr *tknv1.PipelineRun) releasedomain.ManifestStatus {
	if pr == nil {
		return releasedomain.ManifestPending
	}
	cond := succeededCondition(pr.Status.Conditions)
	if cond == nil {
		if pr.Status.StartTime != nil {
			return releasedomain.ManifestRunning
		}
		return releasedomain.ManifestPending
	}
	switch cond.Status {
	case "True":
		return releasedomain.ManifestAvailable
	case "False":
		return releasedomain.ManifestUnavailable
	default:
		if pr.Status.StartTime != nil {
			return releasedomain.ManifestRunning
		}
		return releasedomain.ManifestPending
	}
}

func mapTaskRunStatus(tr *tknv1.TaskRun) releasedomain.StepStatus {
	if tr == nil {
		return releasedomain.StepPending
	}
	cond := succeededCondition(tr.Status.Conditions)
	if cond == nil {
		if tr.Status.StartTime != nil {
			return releasedomain.StepRunning
		}
		return releasedomain.StepPending
	}
	switch cond.Status {
	case "True":
		return releasedomain.StepSucceeded
	case "False":
		return releasedomain.StepFailed
	default:
		if tr.Status.StartTime != nil {
			return releasedomain.StepRunning
		}
		return releasedomain.StepPending
	}
}

func taskRunName(tr *tknv1.TaskRun) string {
	if tr == nil {
		return ""
	}
	if name := strings.TrimSpace(tr.Labels[pipelineTaskLabel]); name != "" {
		return name
	}
	return strings.TrimSpace(tr.Name)
}

func objectKeyString(namespace, name string) string {
	key := pipelineRunObjectKey(namespace, name)
	if key == "" {
		return fmt.Sprintf("%s/%s", strings.TrimSpace(namespace), strings.TrimSpace(name))
	}
	return key
}

func pipelineMessage(pr *tknv1.PipelineRun) string {
	if pr == nil {
		return ""
	}
	cond := succeededCondition(pr.Status.Conditions)
	if cond == nil {
		return ""
	}
	return strings.TrimSpace(cond.Message)
}

func taskRunMessage(tr *tknv1.TaskRun) string {
	if tr == nil {
		return ""
	}
	cond := succeededCondition(tr.Status.Conditions)
	if cond == nil {
		return ""
	}
	return strings.TrimSpace(cond.Message)
}

func pipelineRunStateKey(pr *tknv1.PipelineRun) string {
	if pr == nil {
		return ""
	}
	if ts := pr.Status.CompletionTime; ts != nil {
		return pr.Name + "|" + ts.UTC().Format(time.RFC3339Nano)
	}
	if rv := strings.TrimSpace(pr.ResourceVersion); rv != "" {
		return pr.Name + "|" + rv
	}
	return strings.TrimSpace(pr.Name)
}

type conditionView struct {
	Status  string
	Message string
}

func succeededCondition(conditions duckv1.Conditions) *conditionView {
	for i := range conditions {
		if string(conditions[i].Type) == "Succeeded" {
			return &conditionView{
				Status:  string(conditions[i].Status),
				Message: conditions[i].Message,
			}
		}
	}
	return nil
}
