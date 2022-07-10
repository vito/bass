package buildkitd

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/gofrs/flock"
	_ "github.com/moby/buildkit"
	bk "github.com/moby/buildkit/client"
	_ "github.com/moby/buildkit/client/connhelper/dockercontainer" // import the container connection driver
	"github.com/vito/bass/pkg/zapctx"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

// bumped by hack/bump-buildkit
const Version = "v0.10.3"

const (
	image         = "moby/buildkit"
	containerName = "bass-buildkitd"
	volumeName    = "bass-buildkitd"

	// Long timeout to allow for slow image pulls of
	// buildkitd while not blocking for infinity
	lockTimeout = 10 * time.Minute
)

func Start(ctx context.Context) (string, error) {
	ctx, span := otel.Tracer("bass").Start(ctx, "buildkitd.Start")
	defer span.End()

	if err := checkBuildkit(ctx); err != nil {
		return "", err
	}

	return fmt.Sprintf("docker-container://%s", containerName), nil
}

// ensure the buildkit is active and properly set up (e.g. connected to host and last version with moby/buildkit)
func checkBuildkit(ctx context.Context) error {
	logger := zapctx.FromContext(ctx)

	// acquire a file-based lock to ensure parallel bass clients
	// don't interfere with checking+creating the buildkitd container
	lockFilePath, err := xdg.RuntimeFile("bass/buildkitd.lock")
	if err != nil {
		return fmt.Errorf("unable to expand buildkitd lock path: %w", err)
	}

	lock := flock.New(lockFilePath)

	logger.Debug("acquiring buildkitd lock", zap.String("lockFilePath", lockFilePath))

	lockCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	locked, err := lock.TryLockContext(lockCtx, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to lock buildkitd lock file: %w", err)
	}

	if !locked {
		return fmt.Errorf("failed to acquire buildkitd lock file")
	}

	defer lock.Unlock()

	logger.Debug("acquired buildkitd lock")

	// check status of buildkitd container
	config, err := getBuildkitInformation(ctx)
	if err != nil {
		logger.Debug("failed to get buildkit information", zap.Error(err))

		// If that failed, it might be because the docker CLI is out of service.
		if err := checkDocker(ctx); err != nil {
			return err
		}

		logger.Debug("no buildkit daemon detected")

		if err := removeBuildkit(ctx); err != nil {
			logger.Debug("error while removing buildkit", zap.Error(err))
		}

		if err := installBuildkit(ctx); err != nil {
			return err
		}
	} else {
		logger.Debug("detected buildkit config",
			zap.String("version", config.Version),
			zap.Bool("isActive", config.IsActive),
			zap.Bool("haveHostNetwork", config.HaveHostNetwork))

		if config.Version != Version || !config.HaveHostNetwork {
			logger.Info("upgrading buildkit",
				zap.String("version", Version),
				zap.Bool("have host network", config.HaveHostNetwork))

			if err := removeBuildkit(ctx); err != nil {
				return err
			}
			if err := installBuildkit(ctx); err != nil {
				return err
			}
		}

		if !config.IsActive {
			logger.Info("starting buildkit", zap.String("version", Version))
			if err := startBuildkit(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// ensure the docker CLI is available and properly set up (e.g. permissions to
// communicate with the daemon, etc)
func checkDocker(ctx context.Context) error {
	logger := zapctx.FromContext(ctx)

	cmd := exec.CommandContext(ctx, "docker", "info")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("failed to run docker",
			zap.Error(err),
			zap.ByteString("output", output))
		return err
	}

	return nil
}

// Start the buildkit daemon
func startBuildkit(ctx context.Context) error {
	logger := zapctx.FromContext(ctx).With(zap.String("version", Version))

	logger.Debug("starting buildkit image")

	cmd := exec.CommandContext(ctx,
		"docker",
		"start",
		containerName,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("failed to start buildkit container",
			zap.Error(err),
			zap.ByteString("output", output))
		return err
	}

	return waitBuildkit(ctx)
}

// Pull and run the buildkit daemon with a proper configuration
// If the buildkit daemon is already configured, use startBuildkit
func installBuildkit(ctx context.Context) error {
	logger := zapctx.FromContext(ctx).With(zap.String("version", Version))

	logger.Debug("pulling buildkit image")

	// #nosec
	cmd := exec.CommandContext(ctx,
		"docker",
		"pull",
		image+":"+Version,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("failed to pull buildkit image",
			zap.Error(err),
			zap.ByteString("output", output))
		return err
	}

	// FIXME: buildkitd currently runs without network isolation (--net=host)
	// in order for containers to be able to reach localhost.
	// This is required for things such as kubectl being able to
	// reach a KinD/minikube cluster locally
	// #nosec
	cmd = exec.CommandContext(ctx,
		"docker",
		"run",
		"--net=host",
		"-d",
		"--restart", "always",
		"-v", volumeName+":/var/lib/buildkit",
		"--name", containerName,
		"--privileged",
		image+":"+Version,
		"--debug",
		"--allow-insecure-entitlement", "security.insecure",
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		// If the daemon failed to start because it's already running,
		// chances are another bass instance started it. We can just ignore
		// the error.
		if !strings.Contains(string(output), "Error response from daemon: Conflict.") {
			logger.Error("unable to start buildkitd",
				zap.Error(err),
				zap.ByteString("output", output))
			return err
		}
	}
	return waitBuildkit(ctx)
}

// waitBuildkit waits for the buildkit daemon to be responsive.
func waitBuildkit(ctx context.Context) error {
	c, err := bk.New(ctx, "docker-container://"+containerName)
	if err != nil {
		return err
	}

	// FIXME Does output "failed to wait: signal: broken pipe"
	defer c.Close()

	// Try to connect every 100ms up to 100 times (10 seconds total)
	const (
		retryPeriod   = 100 * time.Millisecond
		retryAttempts = 100
	)

	for retry := 0; retry < retryAttempts; retry++ {
		_, err = c.ListWorkers(ctx)
		if err == nil {
			return nil
		}
		time.Sleep(retryPeriod)
	}
	return errors.New("buildkit failed to respond")
}

func removeBuildkit(ctx context.Context) error {
	logger := zapctx.FromContext(ctx)

	cmd := exec.CommandContext(ctx,
		"docker",
		"rm",
		"-fv",
		containerName,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("unable to stop buildkitd",
			zap.Error(err),
			zap.ByteString("output", output))
		return err
	}

	return nil
}
