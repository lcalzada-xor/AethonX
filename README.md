<p align="center">
  <img src="https://github.com/user-attachments/assets/048eaff9-61c1-4429-aa0a-64d0c18be00f" alt="AethonX Logo" width="480"/>
</p>

<h1 align="center">🧠 AethonX</h1>

<p align="center">
  <b>Engine modular de reconocimiento pasivo y activo</b><br>
  <i>Inspirado en Aethon, uno de los caballos de Helios</i>
</p>

---

**AethonX** es una herramienta profesional de **reconocimiento web** escrita en **Go**, diseñada para automatizar la enumeración de activos y la recopilación de información de forma **pasiva** o **activa**.  
Integra múltiples fuentes en un flujo **orquestado**, **concurrente** y **modular**, permitiendo extender fácilmente nuevas herramientas y fuentes de datos.

> 🐎 El nombre *Aethon* proviene de la mitología griega: uno de los caballos de Helios, el dios del Sol.  
> Al igual que Aethon iluminaba el mundo en su recorrido diario por el cielo, **AethonX** busca arrojar luz sobre los activos expuestos en la superficie digital.

---

## ✨ Características principales

- 🔌 **Arquitectura modular**: cada fuente (`crt.sh`, `RDAP`, etc.) se implementa como módulo independiente.  
- ⚙️ **Orquestador concurrente**: ejecución en paralelo con control de *workers* y manejo de contexto.  
- 📚 **Interfaz unificada (`Source`)**: permite integrar nuevas herramientas fácilmente.  
- 🧩 **Normalización y deduplicación** integradas para datos limpios y consolidados.  
- 🧾 **Salidas flexibles**: tabla en terminal o formato JSON estructurado.  
- 🛠️ **Configuración adaptable**: compatible con *flags*, variables de entorno y perfiles.  
- ⚡ **Diseñada para extensibilidad**: preparada para fases activas (DNSx, HTTPx, etc.) y análisis avanzados.  

---

## 📂 Estructura del proyecto

```
AethonX/
├─ cmd/
│  └─ aethonx/                  # CLI principal (main.go)
├─ internal/
│  ├─ core/                     # Núcleo del pipeline y orquestador
│  ├─ model/                    # Tipos comunes: Artifact, Target, RunResult, etc.
│  ├─ domain/                   # Normalización, dedupe, validaciones
│  ├─ sources/                  # Fuentes (crtsh, rdap, ...)
│  │  ├─ crtsh/
│  │  └─ rdap/
│  ├─ adapters/
│  │  └─ output/                # Salidas (tabla, JSON, futuros formatos)
│  └─ platform/                 # Infraestructura común (config, logx, httpx, ...)
├─ assets/                      # Imágenes, banners, logos
├─ go.mod
├─ go.sum
└─ README.md
```

---

## 🚀 Instalación

### 1️⃣ Clonar el repositorio

```bash
git clone https://github.com/lcalzada-xor/AethonX.git
cd AethonX
```

### 2️⃣ Descargar dependencias

```bash
go mod tidy
```

### 3️⃣ Compilar

```bash
go build -o aethonx ./cmd/aethonx
```

---

## 🧰 Uso

### Ejemplo básico (modo pasivo)

```bash
./aethonx -target example.com
```

### Con salida JSON

```bash
./aethonx -target example.com -out.json -out results/
```

### Control de concurrencia y timeout

```bash
./aethonx -target example.com -workers 8 -timeout 60
```

---

## ⚙️ Variables de entorno

| Variable | Descripción | Ejemplo |
|-----------|--------------|----------|
| `AETHONX_TARGET` | Dominio objetivo | `example.com` |
| `AETHONX_ACTIVE` | Habilitar modo activo | `true` |
| `AETHONX_WORKERS` | Máx. concurrencia | `8` |
| `AETHONX_TIMEOUT` | Timeout global (s) | `45` |
| `AETHONX_OUTPUT_DIR` | Directorio de salida | `./out` |
| `AETHONX_SOURCES_CRTSH` | Activar/desactivar crt.sh | `false` |
| `AETHONX_SOURCES_RDAP` | Activar/desactivar RDAP | `true` |

---

## 🧩 Flujo interno

```
config.Load()  →  logger.New()  →  orchestrator.Run()
          ↳ sources (crt.sh, RDAP, …)
              ↳ artifacts[]
          ↳ domain.DedupeAndNormalize()
          ↳ output.(table|json)
```

---

## 🔧 Añadir nuevas fuentes

Crea una carpeta bajo `internal/sources/<tool>` e implementa la interfaz:

```go
type Source interface {
    Name() string
    Mode() model.SourceMode
    Run(ctx context.Context, t model.Target) (model.RunResult, error)
}
```

Ejemplo:
```go
func (s *MyTool) Run(ctx context.Context, t model.Target) (model.RunResult, error) {
    // Lógica para obtener subdominios, IPs, etc.
    return model.RunResult{Artifacts: artifacts}, nil
}
```

Luego regístrala en `buildSources()` dentro de `cmd/aethonx/main.go`.

---

## 🧠 Roadmap

| Fase | Funcionalidad | Estado |
|------|----------------|--------|
| 1️⃣ | Núcleo modular (core, config, logx) | ✅ |
| 2️⃣ | Fuentes pasivas: `crt.sh`, `RDAP` | ✅ |
| 3️⃣ | Dedupe + Salidas JSON/Table | ✅ |
| 4️⃣ | Infra `httpx` con proxy, retry, cache | ⏳ |
| 5️⃣ | Fuentes activas: `dnsx`, `httpx`, `subjs` | 🧩 |
| 6️⃣ | Reporting (Markdown, HTML, SARIF) | 🚧 |
| 7️⃣ | CLI avanzada con subcomandos | 🚧 |

---

## 🧑‍💻 Autor

**Lucas Calzada**  
💼 Cybersecurity Engineer | Developer | Researcher  
📍 España  
🔗 [GitHub](https://github.com/lcalzada-xor)

---

## 📜 Licencia

Este proyecto se distribuye bajo licencia **MIT**.  
Consulta el archivo [LICENSE](LICENSE) para más detalles.
