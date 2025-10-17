// internal/core/ports/notifier.go
package ports

import (
	"context"
	"time"

	"aethonx/internal/core/domain"
)

// Notifier es el port para notificaciones de eventos del sistema.
// Implementa el patrón Observer para desacoplar la lógica de negocio
// de los mecanismos de notificación (webhooks, Slack, etc.).
type Notifier interface {
	// Notify envía una notificación para un evento
	Notify(ctx context.Context, event Event) error

	// Close cierra el notifier y libera recursos
	Close() error
}

// AsyncNotifier permite notificaciones asíncronas sin bloquear.
type AsyncNotifier interface {
	Notifier

	// NotifyAsync envía una notificación de forma asíncrona
	NotifyAsync(event Event)
}

// FilteredNotifier permite filtrar qué eventos se notifican.
type FilteredNotifier interface {
	Notifier

	// SetFilter configura un filtro de eventos
	SetFilter(filter EventFilter)
}

// Event representa un evento del sistema.
type Event struct {
	// Type tipo de evento
	Type EventType

	// Timestamp momento del evento
	Timestamp time.Time

	// Source fuente que generó el evento
	Source string

	// Target objetivo relacionado (opcional)
	Target string

	// Data datos específicos del evento
	Data interface{}

	// Severity severidad del evento
	Severity EventSeverity

	// Metadata información adicional
	Metadata map[string]string
}

// EventType define los tipos de eventos del sistema.
type EventType string

const (
	// Scan events
	EventTypeScanStarted   EventType = "scan.started"
	EventTypeScanCompleted EventType = "scan.completed"
	EventTypeScanFailed    EventType = "scan.failed"
	EventTypeScanCanceled  EventType = "scan.canceled"

	// Source events
	EventTypeSourceStarted   EventType = "source.started"
	EventTypeSourceCompleted EventType = "source.completed"
	EventTypeSourceFailed    EventType = "source.failed"
	EventTypeSourceTimeout   EventType = "source.timeout"

	// Artifact events
	EventTypeArtifactDiscovered EventType = "artifact.discovered"
	EventTypeArtifactValidated  EventType = "artifact.validated"

	// System events
	EventTypeSystemError   EventType = "system.error"
	EventTypeSystemWarning EventType = "system.warning"
)

// EventSeverity define la severidad de un evento.
type EventSeverity string

const (
	EventSeverityInfo     EventSeverity = "info"
	EventSeverityWarning  EventSeverity = "warning"
	EventSeverityError    EventSeverity = "error"
	EventSeverityCritical EventSeverity = "critical"
)

// EventFilter filtra eventos basados en criterios.
type EventFilter struct {
	// Types tipos de eventos a incluir (vacío = todos)
	Types []EventType

	// Severities severidades a incluir (vacío = todas)
	Severities []EventSeverity

	// Sources fuentes específicas (vacío = todas)
	Sources []string

	// MinSeverity severidad mínima para incluir
	MinSeverity EventSeverity
}

// NewEvent crea un nuevo evento.
func NewEvent(eventType EventType, source string, data interface{}) Event {
	return Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Source:    source,
		Data:      data,
		Severity:  EventSeverityInfo,
		Metadata:  make(map[string]string),
	}
}

// ScanStartedEvent datos para evento de inicio de escaneo.
type ScanStartedEvent struct {
	ScanID string
	Target domain.Target
}

// ScanCompletedEvent datos para evento de finalización de escaneo.
type ScanCompletedEvent struct {
	ScanID         string
	Target         domain.Target
	ArtifactsCount int
	Duration       time.Duration
}

// ArtifactDiscoveredEvent datos para evento de descubrimiento de artifact.
type ArtifactDiscoveredEvent struct {
	Artifact *domain.Artifact
	ScanID   string
}

// NotifierFactory es una función que crea una instancia de Notifier.
type NotifierFactory func(config map[string]interface{}) (Notifier, error)
