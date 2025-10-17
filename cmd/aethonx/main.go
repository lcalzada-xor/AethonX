// cmd/aethonx/main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aethonx/internal/core"
	"aethonx/internal/platform/config"
	"aethonx/internal/platform/logx"

	"aethonx/internal/adapters/output"

	// Fuentes pasivas iniciales
	"aethonx/internal/sources/crtsh"
)

var (
	// Rellenables con -ldflags en build
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// 1. Cargar config centralizada
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(2)
	}
	if cfg.PrintVersion {
		fmt.Printf("AethonX %s (commit %s, built %s)\n", version, commit, date)
		return
	}
	if cfg.Target == "" {
		fmt.Fprintln(os.Stderr, "missing -target, try: aethonx -target example.com")
		os.Exit(2)
	}

	// 2. Logger compartido
	logger := logx.New() // asume API mínima Info/Debug/Warn/Err

	logger.Info("aethonx starting",
		"version", version,
		"commit", commit,
		"date", date,
		"target", cfg.Target,
		"active", cfg.Active,
		"workers", cfg.Workers,
	)

	// 3. Contexto y señales para shutdown limpio
	ctx, cancel := rootContextWithSignals(cfg.TimeoutS)
	defer cancel()

	// 4. Registrar fuentes según config
	sources := buildSources(logger, cfg)

	if len(sources) == 0 {
		logger.Err(errors.New("no sources enabled"))
		os.Exit(2)
	}

	// 5. Orquestador central
	orch := core.Orchestrator{
		Sources: sources,
		Limit:   max(1, cfg.Workers),
	}

	// 6. Ejecutar flujo
	start := time.Now()
	res, runErr := orch.Run(ctx, core.Target{
		RootDomain: cfg.Target,
		Active:     cfg.Active,
	})
	elapsed := time.Since(start)

	// 7. Manejo de error de ejecución
	if runErr != nil {
		logger.Err(runErr, "phase", "run", "elapsed_ms", elapsed.Milliseconds())
		// Continuamos para emitir lo que haya, útil en pipelines
	}

	// 8. Salidas
	outErr := writeOutputs(cfg, res)
	if outErr != nil {
		logger.Err(outErr, "phase", "output")
		// Si la salida principal falla, salimos con código distinto
		os.Exit(1)
	}

	// 9. Resumen
	logger.Info("aethonx finished",
		"elapsed_ms", elapsed.Milliseconds(),
		"artifacts", len(res.Artifacts),
		"warnings", len(res.Warnings),
	)
	if runErr != nil {
		os.Exit(1)
	}
}

// buildSources registra las fuentes disponibles respetando toggles de config.
// Mantén esta función como único punto de ensamblaje para nuevas herramientas.
func buildSources(logger logx.Logger, cfg config.Config) []core.Source {
	var ss []core.Source

	// Ejemplos de toggles. Si no existen en tu config, elimina los checks y registra por defecto.
	if cfg.Sources.CRTSHEnabled {
		ss = append(ss, crtsh.New(logger))
	}
	if cfg.Sources.RDAPEnabled {
		ss = append(ss, rdap.New(logger))
	}

	// Aquí podrás añadir más fuentes sin tocar main
	// if cfg.Sources.SubfinderEnabled { ss = append(ss, subfinder.New(logger, cfg.Platform)) }
	// if cfg.Sources.AmassEnabled { ss = append(ss, amass.New(logger, cfg.Platform, cfg.Active)) }
	// ...

	return ss
}

// writeOutputs decide y ejecuta las salidas según config.
// Mantener aislado del main facilita añadir nuevos formatos sin tocar el flujo.
func writeOutputs(cfg config.Config, res core.RunResult) error {
	// Prioridad a JSON si se pide explícitamente
	if cfg.Outputs.JSONEnabled {
		if err := output.OutputJSON(cfg.OutputDir, res); err != nil {
			return fmt.Errorf("json output: %w", err)
		}
	}

	// Tabla legible por terminal si no se desactiva
	if !cfg.Outputs.TableDisabled {
		if err := output.OutputTable(res); err != nil {
			return fmt.Errorf("table output: %w", err)
		}
	}

	// Aquí puedes encadenar otros adaptadores: NDJSON, Markdown, SARIF, etc.
	// if cfg.Outputs.NDJSONEnabled { ... }

	return nil
}

// rootContextWithSignals crea un contexto raíz con timeout opcional y cancelación por señal.
func rootContextWithSignals(timeoutSeconds int) (context.Context, context.CancelFunc) {
	var base context.Context = context.Background()
	var cancel context.CancelFunc

	if timeoutSeconds > 0 {
		base, cancel = context.WithTimeout(base, time.Duration(timeoutSeconds)*time.Second)
	} else {
		base, cancel = context.WithCancel(base)
	}

	// Cancelación por SIGINT/SIGTERM
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()

	return base, cancel
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
