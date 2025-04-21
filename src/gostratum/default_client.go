package gostratum

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/pkg/errors"
	"github.com/spectre-project/spectre-stratum-bridge/src/utils"
	"github.com/spectre-project/spectred/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type StratumMethod string

const (
	StratumMethodSubscribe           StratumMethod = "mining.subscribe"
	StratumMethodExtranonceSubscribe StratumMethod = "mining.extranonce.subscribe"
	StratumMethodAuthorize           StratumMethod = "mining.authorize"
	StratumMethodSubmit              StratumMethod = "mining.submit"
)

func DefaultLogger() *zap.Logger {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		&utils.BufferedWriteSyncer{WS: zapcore.AddSync(colorable.NewColorableStdout()), FlushInterval: 5 * time.Second},
		zapcore.DebugLevel,
	))
}

func DefaultConfig(logger *zap.Logger) StratumListenerConfig {
	return StratumListenerConfig{
		StateGenerator: func() any { return nil },
		HandlerMap:     DefaultHandlers(),
		Port:           ":5555",
		Logger:         logger,
	}
}

func DefaultHandlers() StratumHandlerMap {
	return StratumHandlerMap{
		string(StratumMethodSubscribe):           HandleSubscribe,
		string(StratumMethodExtranonceSubscribe): HandleExtranonceSubscribe,
		string(StratumMethodAuthorize):           HandleAuthorize,
		string(StratumMethodSubmit):              HandleSubmit,
	}
}

func HandleAuthorize(ctx *StratumContext, event JsonRpcEvent) error {
	if len(event.Params) < 1 {
		return fmt.Errorf("malformed event from miner, expected param[1] to be address")
	}
	address, ok := event.Params[0].(string)
	if !ok {
		return fmt.Errorf("malformed event from miner, expected param[1] to be address string")
	}
	parts := strings.Split(address, ".")
	var workerName string
	if len(parts) >= 2 {
		address = parts[0]
		workerName = parts[1]
	}
	var err error
	address, err = CleanWallet(address)
	if err != nil {
		return fmt.Errorf("invalid wallet format %s: %w", address, err)
	}

	ctx.WalletAddr = address
	ctx.WorkerName = workerName
	ctx.Logger = ctx.Logger.With(zap.String("worker", ctx.WorkerName), zap.String("addr", ctx.WalletAddr))

	if err := ctx.Reply(NewResponse(event, true, nil)); err != nil {
		return errors.Wrap(err, "failed to send response to authorize")
	}
	if ctx.Extranonce != "" {
		SendExtranonce(ctx)
	}

	ctx.Logger.Info(fmt.Sprintf("client authorized, address: %s", ctx.WalletAddr))
	return nil
}

func HandleSubscribe(ctx *StratumContext, event JsonRpcEvent) error {
	if err := ctx.Reply(NewResponse(event,
		[]any{true, "EthereumStratum/1.0.0"}, nil)); err != nil {
		return errors.Wrap(err, "failed to send response to subscribe")
	}
	if len(event.Params) > 0 {
		app, ok := event.Params[0].(string)
		if ok {
			ctx.RemoteApp = app
		}
	}

	ctx.Logger.Info("client subscribed ", zap.Any("context", ctx))
	return nil
}

func HandleExtranonceSubscribe(ctx *StratumContext, event JsonRpcEvent) error {
	err := ctx.Reply(NewResponse(event, true, nil))
	if err != nil {
		return errors.Wrap(err, "failed to send response to extranonce subscribe")
	}

	ctx.Logger.Info("client subscribed to extranonce ", zap.Any("context", ctx))
	return nil
}

func HandleSubmit(ctx *StratumContext, event JsonRpcEvent) error {
	// stub
	ctx.Logger.Info("work submission")
	return nil
}

func SendExtranonce(ctx *StratumContext) {
	if err := ctx.Send(NewEvent("", "set_extranonce", []any{ctx.Extranonce})); err != nil {
		// should we doing anything further on failure
		ctx.Logger.Error(errors.Wrap(err, "failed to set extranonce").Error(), zap.Any("context", ctx))
	}
}

var walletRegex = regexp.MustCompile("(spectre|spectretest|spectredev):[a-z0-9]+")

func CleanWallet(in string) (string, error) {
	if !strings.HasPrefix(in, "spectre:") &&
		!strings.HasPrefix(in, "spectretest:") &&
		!strings.HasPrefix(in, "spectredev:") {
		in = "spectre:" + in
	}

	// skip verification for spectredev address
	if strings.HasPrefix(in, "spectredev:") {
		return in, nil
	}

	if _, err := util.DecodeAddress(in, util.Bech32PrefixSpectre); err == nil {
		return in, nil // valid spectre address
	}
	if _, err := util.DecodeAddress(in, util.Bech32PrefixSpectreTest); err == nil {
		return in, nil // valid spectretest address
	}

	return "", errors.New("invalid wallet address")
}
