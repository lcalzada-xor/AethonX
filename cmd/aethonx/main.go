// cmd/aethonx/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aethonx/internal/adapters/output"
	"aethonx/internal/core/domain"
	"aethonx/internal/core/ports"
	"aethonx/internal/core/usecases"
	"aethonx/internal/platform/config"
	"aethonx/internal/platform/logx"
	"aethonx/internal/sources/crtsh"
	"aethonx/internal/sources/rdap"
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

	// Version flag
	if cfg.PrintVersion {
		fmt.Printf("AethonX %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	// Validar target
	if cfg.Target == "" {
		fmt.Fprintln(os.Stderr, "missing -target, try: aethonx -target example.com")
		os.Exit(2)
	}

	// 2. Logger compartido
	logger := logx.New()

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

	// 4. Construir target del dominio
	scanMode := domain.ScanModePassive
	if cfg.Active {
		scanMode = domain.ScanModeActive
	}

	target := domain.NewTarget(cfg.Target, scanMode)

	// Validar target
	if err := target.Validate(); err != nil {
		logger.Err(err, "phase", "validation")
		os.Exit(2)
	}

	// 5. Registrar fuentes según config
	sources := buildSources(logger, cfg)

	if len(sources) == 0 {
		logger.Err(fmt.Errorf("no sources enabled"))
		os.Exit(2)
	}

	logger.Info("sources registered", "count", len(sources))

	// 6. Crear orquestador
	orch := usecases.NewOrchestrator(usecases.OrchestratorOptions{
		Sources:    sources,
		Logger:     logger,
		Observers:  []ports.Notifier{}, // Futuro: webhooks, metrics, etc.
		MaxWorkers: max(1, cfg.Workers),
		FailFast:   false,
	})

	// 7. Ejecutar flujo
	start := time.Now()
	result, runErr := orch.Run(ctx, *target)
	elapsed := time.Since(start)

	// Añadir metadata de versión
	if result != nil {
		result.Metadata.Version = version
		result.Metadata.Environment = map[string]string{
			"commit": commit,
			"date":   date,
		}
	}

	// 8. Manejo de error de ejecución
	if runErr != nil {
		logger.Err(runErr, "phase", "run", "elapsed_ms", elapsed.Milliseconds())
		// Continuamos para emitir lo que haya, útil en pipelines
	}

	// 9. Salidas
	if result != nil {
		outErr := writeOutputs(cfg, result)
		if outErr != nil {
			logger.Err(outErr, "phase", "output")
			os.Exit(1)
		}
	}

	// 10. Resumen
	if result != nil {
		logger.Info("aethonx finished",
			"elapsed_ms", elapsed.Milliseconds(),
			"artifacts", result.TotalArtifacts(),
			"warnings", len(result.Warnings),
			"errors", len(result.Errors),
		)
	}

	if runErr != nil {
		os.Exit(1)
	}
}

// buildSources registra las fuentes disponibles respetando toggles de config.
// Mantén esta función como único punto de ensamblaje para nuevas herramientas.
func buildSources(logger logx.Logger, cfg config.Config) []ports.Source {
	var sources []ports.Source

	// Registrar crt.sh si está habilitada
	if cfg.Sources.CRTSHEnabled {
		sources = append(sources, crtsh.New(logger))
		logger.Debug("registered source", "name", "crtsh")
	}

	// Registrar RDAP si está habilitada
	if cfg.Sources.RDAPEnabled {
		sources = append(sources, rdap.New(logger))
		logger.Debug("registered source", "name", "rdap")
	}

	// Futuras fuentes:
	// if cfg.Sources.SubfinderEnabled {
	//     sources = append(sources, subfinder.New(logger, cfg.Platform))
	// }
	// if cfg.Sources.AmassEnabled {
	//     sources = append(sources, amass.New(logger, cfg.Platform, cfg.Active))
	// }

	return sources
}

// writeOutputs decide y ejecuta las salidas según config.
// Mantener aislado del main facilita añadir nuevos formatos sin tocar el flujo.
func writeOutputs(cfg config.Config, result *domain.ScanResult) error {
	// Prioridad a JSON si se pide explícitamente
	if cfg.Outputs.JSONEnabled {
		if err := output.OutputJSON(cfg.OutputDir, result); err != nil {
			return fmt.Errorf("json output: %w", err)
		}
	}

	// Tabla legible por terminal si no se desactiva
	if !cfg.Outputs.TableDisabled {
		if err := output.OutputTable(result); err != nil {
			return fmt.Errorf("table output: %w", err)
		}
	}

	// Aquí puedes encadenar otros adaptadores: NDJSON, Markdown, SARIF, etc.
	// if cfg.Outputs.NDJSONEnabled { ... }
	// if cfg.Outputs.MarkdownEnabled { ... }

	return nil
}

// rootContextWithSignals crea un contexto raíz con timeout opcional y cancelación por señal.
func rootContextWithSignals(timeoutSeconds int) (context.Context, context.CancelFunc) {
	var base context.Context
	var cancel context.CancelFunc

	if timeoutSeconds > 0 {
		base, cancel = context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	} else {
		base, cancel = context.WithCancel(context.Background())
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
