# Arquitectura de AethonX

## 📐 Diseño General

AethonX sigue los principios de **Clean Architecture** / **Hexagonal Architecture**, separando claramente las responsabilidades en capas:

```
┌─────────────────────────────────────────────────────┐
│                    Entrypoint                        │
│                   (cmd/aethonx)                      │
└──────────────────────┬──────────────────────────────┘
                       │
┌──────────────────────┴──────────────────────────────┐
│              Adapters (I/O Layer)                    │
│    ┌──────────────┐           ┌─────────────┐       │
│    │   Sources    │           │   Outputs   │       │
│    │ (crtsh, etc) │           │ (json, etc) │       │
│    └──────────────┘           └─────────────┘       │
└──────────────────────┬──────────────────────────────┘
                       │
┌──────────────────────┴──────────────────────────────┐
│              Core (Business Logic)                   │
│                                                      │
│  ┌─────────────────────────────────────────────┐   │
│  │             Ports (Interfaces)              │   │
│  │  - Source                                   │   │
│  │  - Exporter                                 │   │
│  │  - Repository                               │   │
│  │  - Notifier                                 │   │
│  └─────────────────────────────────────────────┘   │
│                       │                              │
│  ┌─────────────────────────────────────────────┐   │
│  │      Use Cases (Application Services)       │   │
│  │  - Orchestrator                             │   │
│  │  - DedupeService                            │   │
│  └─────────────────────────────────────────────┘   │
│                       │                              │
│  ┌─────────────────────────────────────────────┐   │
│  │           Domain (Entities)                 │   │
│  │  - Artifact                                 │   │
│  │  - Target                                   │   │
│  │  - ScanResult                               │   │
│  └─────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────┘
                       │
┌──────────────────────┴──────────────────────────────┐
│            Platform (Infrastructure)                 │
│    ┌────────────┐  ┌────────────┐  ┌──────────┐    │
│    │   Logger   │  │   Config   │  │ HTTPClient│   │
│    └────────────┘  └────────────┘  └──────────┘    │
└──────────────────────────────────────────────────────┘
```

---

## 🏗️ Estructura de Carpetas

```
AethonX/
├── cmd/
│   └── aethonx/              # Entrypoint principal
│       └── main.go
│
├── internal/
│   ├── core/                 # ⚙️ NÚCLEO (Clean Architecture)
│   │   ├── domain/           # Entidades de negocio
│   │   │   ├── artifact.go
│   │   │   ├── target.go
│   │   │   ├── scan_result.go
│   │   │   ├── enums.go
│   │   │   └── errors.go
│   │   │
│   │   ├── ports/            # Interfaces (Hexagonal)
│   │   │   ├── source.go
│   │   │   ├── exporter.go
│   │   │   ├── repository.go
│   │   │   └── notifier.go
│   │   │
│   │   └── usecases/         # Casos de uso
│   │       ├── orchestrator.go
│   │       └── dedupe_service.go
│   │
│   ├── sources/              # 🔌 Implementaciones de fuentes
│   │   └── crtsh/
│   │       └── crtsh.go
│   │
│   ├── adapters/             # 🔄 Adaptadores I/O
│   │   └── output/
│   │       ├── json.go
│   │       └── table.go
│   │
│   └── platform/             # 🏗️ Infraestructura
│       ├── config/
│       │   └── config.go
│       └── logx/
│           └── logx.go
│
├── docs/                     # 📚 Documentación
│   └── ARCHITECTURE.md
│
├── Makefile                  # Comandos de build
├── .golangci.yml             # Configuración de linters
└── go.mod
```

---

## 🎯 Capas de la Arquitectura

### 1. **Domain Layer** (`internal/core/domain/`)

Contiene las **entidades de negocio** puras, sin dependencias externas.

#### Entidades principales:

- **`Artifact`**: Representa un dato descubierto (subdomain, IP, email, etc.)
  - Métodos: `Normalize()`, `Merge()`, `IsValid()`, `Key()`
  - Genera ID único mediante hash SHA256
  - Confianza (0.0-1.0), sources, metadata

- **`Target`**: Objetivo del reconocimiento
  - Dominio root, modo de escaneo, scope
  - Validación de formato de dominio
  - Método `IsInScope()` para filtrado

- **`ScanResult`**: Resultado agregado de un escaneo
  - Lista de artifacts, warnings, errors
  - Metadata (duración, fuentes usadas, timestamps)
  - Métodos: `Stats()`, `Finalize()`, `Summary()`

- **`Enums`**: Tipos enumerados
  - `ScanMode`: Passive, Active, Hybrid
  - `SourceMode`: Passive, Active, Both
  - `SourceType`: API, CLI, Builtin, File, Database
  - `ArtifactType`: Domain, Subdomain, IP, Email, etc.

---

### 2. **Ports Layer** (`internal/core/ports/`)

Define las **interfaces** (contratos) que deben cumplir los adaptadores.

#### Ports principales:

- **`Source`**: Interface que deben implementar todas las fuentes
  ```go
  type Source interface {
      Name() string
      Mode() domain.SourceMode
      Type() domain.SourceType
      Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error)
  }
  ```

- **`Exporter`**: Interface para exportar resultados
  ```go
  type Exporter interface {
      Name() string
      SupportedFormats() []string
      Export(result *domain.ScanResult, opts ExportOptions) error
  }
  ```

- **`Repository`**: Interface para persistencia (futuro)
- **`Notifier`**: Interface para notificaciones/eventos (futuro)

#### Extensiones opcionales:
- `AdvancedSource`: Initialize, Validate, Close, HealthCheck
- `StreamingSource`: Emisión en tiempo real
- `RateLimitedSource`: Control de rate limiting

---

### 3. **Use Cases Layer** (`internal/core/usecases/`)

Contiene la **lógica de aplicación** que orquesta las entidades y ports.

#### Casos de uso:

- **`Orchestrator`**: Coordina ejecución de múltiples fuentes
  - Ejecución concurrente con límite de workers
  - Filtrado de fuentes compatibles con el modo de escaneo
  - Consolidación de resultados
  - Notificación de eventos

- **`DedupeService`**: Normalización y deduplicación
  - Elimina artifacts duplicados
  - Merge de metadata y sources
  - Filtrado por tipo, confianza, fuente
  - Agrupación y estadísticas

---

### 4. **Adapters Layer** (`internal/sources/`, `internal/adapters/`)

Implementaciones concretas de las interfaces (ports).

#### Sources (Adaptadores de entrada):
- **`crtsh`**: Consulta certificados SSL/TLS
  - Tipo: API
  - Modo: Passive
  - Extrae subdominios de certificados públicos

#### Outputs (Adaptadores de salida):
- **`json`**: Exporta a JSON con timestamp
- **`table`**: Tabla legible en terminal con estadísticas

---

### 5. **Platform Layer** (`internal/platform/`)

Servicios de **infraestructura** compartidos.

- **`config`**: Carga configuración desde ENV, flags, archivos
- **`logx`**: Logger estructurado con niveles (Debug, Info, Warn, Err)
- **`httpclient`**: (Futuro) Cliente HTTP con retry, rate limit, proxy
- **`cache`**: (Futuro) Cache genérico

---

## 🔄 Flujo de Ejecución

```
1. main.go carga configuración
         ↓
2. Crea Target del dominio
         ↓
3. Valida Target
         ↓
4. Registra Sources según config
         ↓
5. Crea Orchestrator con Sources
         ↓
6. Orchestrator.Run(ctx, target)
         ├─→ Filtra fuentes compatibles
         ├─→ Ejecuta fuentes en paralelo
         ├─→ Cada fuente retorna ScanResult
         ├─→ Consolida resultados
         └─→ Deduplica artifacts
         ↓
7. Exporta resultado (JSON/Table)
         ↓
8. Log de resumen y exit
```

---

## 🎨 Patrones de Diseño Aplicados

| Patrón | Ubicación | Propósito |
|--------|-----------|-----------|
| **Hexagonal Architecture** | Core + Ports | Independencia de frameworks |
| **Repository** | `ports/repository.go` | Abstracción de persistencia |
| **Strategy** | `ports/source.go` | Diferentes fuentes intercambiables |
| **Factory** | `buildSources()` en main | Construcción de fuentes |
| **Observer** | `ports/notifier.go` | Notificaciones de eventos |
| **Builder** | `domain.NewTarget()` | Construcción de entidades |
| **Dependency Injection** | Orchestrator options | Desacoplamiento |

---

## ✅ Principios SOLID

### Single Responsibility Principle (SRP)
- Cada entidad tiene una responsabilidad única
- `Orchestrator`: solo orquesta
- `DedupeService`: solo deduplica
- `Artifact`: solo representa datos descubiertos

### Open/Closed Principle (OCP)
- Abierto a extensión (nuevas fuentes) sin modificar código existente
- Se añaden fuentes implementando `ports.Source`

### Liskov Substitution Principle (LSP)
- Cualquier implementación de `Source` puede sustituir a otra
- Todos los exporters son intercambiables

### Interface Segregation Principle (ISP)
- Interfaces pequeñas y específicas
- `StreamingSource`, `RateLimitedSource` son opcionales

### Dependency Inversion Principle (DIP)
- Core depende de abstracciones (ports), no de implementaciones
- Inyección de dependencias via constructores

---

## 🚀 Ventajas de Esta Arquitectura

### ✅ **Mantenibilidad**
- Código organizado en capas claras
- Fácil de navegar y entender
- Cambios localizados

### ✅ **Extensibilidad**
- Añadir nueva fuente: 1 archivo + implementar `ports.Source`
- Añadir nuevo output: 1 archivo + implementar `ports.Exporter`
- Sin cambios en el core

### ✅ **Testabilidad**
- Interfaces facilitan mocks
- Domain entities son puras (sin I/O)
- Use cases se prueban con fuentes fake

### ✅ **Escalabilidad**
- Orquestador concurrente con límite de workers
- Deduplicación eficiente con mapas
- Streaming opcional para grandes volúmenes

### ✅ **Independencia**
- Core no depende de frameworks externos
- Fácil migrar de logger, config, HTTP client
- Portable a otros entornos (CLI, API, Lambda)

---

## 📝 Cómo Añadir una Nueva Fuente

### Ejemplo: Agregar Shodan

```go
// internal/sources/shodan/shodan.go
package shodan

import (
    "context"
    "aethonx/internal/core/domain"
    "aethonx/internal/core/ports"
)

type Shodan struct {
    apiKey string
    logger logx.Logger
}

func New(logger logx.Logger, apiKey string) ports.Source {
    return &Shodan{
        apiKey: apiKey,
        logger: logger.With("source", "shodan"),
    }
}

func (s *Shodan) Name() string {
    return "shodan"
}

func (s *Shodan) Mode() domain.SourceMode {
    return domain.SourceModePassive
}

func (s *Shodan) Type() domain.SourceType {
    return domain.SourceTypeAPI
}

func (s *Shodan) Run(ctx context.Context, target domain.Target) (*domain.ScanResult, error) {
    result := domain.NewScanResult(target)

    // 1. Consultar API de Shodan
    // 2. Parsear respuesta
    // 3. Crear artifacts
    // 4. Retornar resultado

    return result, nil
}
```

### Registrar en main.go:

```go
if cfg.Sources.ShodanEnabled {
    sources = append(sources, shodan.New(logger, cfg.Sources.ShodanAPIKey))
}
```

¡Listo! Sin tocar el core, orchestrator, ni outputs.

---

## 🔮 Roadmap de Arquitectura

### Fase 2: Infraestructura Robusta
- [ ] `platform/httpclient` con retry, rate limit, proxy
- [ ] `platform/logger` con slog (structured logging)
- [ ] `platform/cache` para evitar re-queries

### Fase 3: Sources Registry
- [ ] Factory pattern para auto-registro
- [ ] Configuración dinámica de fuentes
- [ ] Base classes para reducir boilerplate

### Fase 4: Persistencia
- [ ] Implementar `ports.Repository`
- [ ] Adapter SQLite para históricos
- [ ] Adapter PostgreSQL para producción

### Fase 5: Observabilidad
- [ ] Implementar `ports.Notifier`
- [ ] Webhooks para eventos
- [ ] Métricas con Prometheus

### Fase 6: API REST
- [ ] Servidor HTTP en `cmd/aethonx-server`
- [ ] Endpoints REST para scans
- [ ] WebSockets para streaming

---

## 📚 Referencias

- [Clean Architecture - Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Hexagonal Architecture - Alistair Cockburn](https://alistair.cockburn.us/hexagonal-architecture/)
- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [SOLID Principles in Go](https://dave.cheney.net/2016/08/20/solid-go-design)
