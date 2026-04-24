package pyroscopex

import (
	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/grafana/pyroscope-go"
	"go.uber.org/zap"
)

func InitPyroscope(name, address string) {
	if _, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: name,
		ServerAddress:   address,
		Logger:          logger.NewZapAdapter(logger.Logger),
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
	}); err != nil {
		logger.Logger.Warn("pyroscope initialization failed", zap.Error(err))
	}
}
