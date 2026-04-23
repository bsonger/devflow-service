package pyroscopex

import (
	"github.com/bsonger/devflow-service/shared/loggingx"
	"github.com/grafana/pyroscope-go"
)

func InitPyroscope(name, address string) {
	pyroscope.Start(pyroscope.Config{
		ApplicationName: name,
		ServerAddress:   address,
		Logger:          loggingx.NewZapAdapter(loggingx.Logger),
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockDuration,
			pyroscope.ProfileBlockCount,
		},
	})
}
