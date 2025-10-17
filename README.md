<p align="center">
  <img src="https://github.com/user-attachments/assets/a97c4c9b-f850-461f-bb53-35c5e660327e" alt="AethonX Logo" width="480"/>
</p>

<h1 align="center">🧠 AethonX</h1>

<p align="center">
  <b>Engine modular de reconocimiento pasivo y activo</b><br>
  <i>Inspirado en Aethon, el caballo de Helios</i>
</p>

---

**AethonX** es una herramienta de reconocimiento escrita en **Go**, diseñada para enumerar información de aplicaciones web de forma **pasiva** o **activa**, integrando múltiples fuentes en un flujo **orquestado**, **concurrente** y **extensible**.


> ⚔️ Inspirada en el titán y dragón solar **Aethon**, símbolo de la vigilancia constante:  
> AethonX ilumina los activos ocultos y expone la superficie de ataque.

---

## ✨ Características

- 🔌 **Arquitectura modular**: cada fuente (`crt.sh`, `RDAP`, etc.) es un módulo independiente.  
- ⚙️ **Orquestador concurrente**: ejecuta fuentes en paralelo con control de *workers*.  
- 📚 **Interfaz común (`Source`)**: facilita añadir nuevas herramientas.  
- 🧩 **Dedupe + normalización** integrados.  
- 🧾 **Salidas personalizables**: tabla en terminal y JSON estructurado.  
- 🛠️ **Configuración flexible** vía *flags* o variables de entorno.  
- ⚡ **Preparado para fases activas** (DNSx, HTTPx, etc.) y reporting avanzado.  

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

### 2️⃣ Inicializar y descargar dependencias

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

### Ejecutar con más concurrencia y tiempo de espera personalizado

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
