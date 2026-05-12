package observer

const (
	ObserveStateLabel        = "devflow.io/observe-state"
	ObserveKindAnnotation    = "devflow.io/observe-kind"
	ObserveOwnerIDAnnotation = "devflow.io/observe-owner-id"
)

const (
	ObserveStateRunning = "running"
	ObserveStateDone    = "done"
)

const (
	ObserveKindRelease  = "release"
	ObserveKindManifest = "manifest"
)
