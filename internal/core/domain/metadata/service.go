// internal/core/domain/metadata/service.go
package metadata

import (
	"strconv"
	"strings"
)

// ServiceMetadata contiene información detallada sobre un servicio de red (Nmap/Masscan).
type ServiceMetadata struct {
	// Identificación del servicio
	Name      string // "mysql", "ssh", "http", "ftp", "smtp"
	Product   string // "MySQL", "OpenSSH", "nginx", "vsftpd"
	Version   string // "5.7.40", "8.9p1", "1.24.0"
	ExtraInfo string // "Ubuntu Linux; protocol 2.0"

	// Puerto y protocolo
	Port     int    // 3306, 22, 80, 21, 25
	Protocol string // "tcp", "udp"
	State    string // "open", "filtered", "closed"

	// Banner y fingerprinting
	Banner    string // Banner raw capturado
	ServiceFP string // Fingerprint del servicio

	// CPE (Common Platform Enumeration)
	CPE string // "cpe:/a:mysql:mysql:5.7.40"

	// SSL/TLS (si el servicio usa SSL)
	SSLEnabled bool
	SSLCert    string // Subject del certificado

	// Vulnerabilidades conocidas
	HasVulns  bool
	CVEList   []string
	RiskLevel string // "low", "medium", "high", "critical"

	// Script results (Nmap NSE scripts)
	ScriptResults map[string]string // script_name -> output

	// Detección
	DetectionMethod string  // "banner", "probe", "inference"
	Confidence      float64 // 0.0-1.0
	ScanTool        string  // "nmap", "masscan", "naabu"

	// Relación con IP
	ParentIP string // IP donde se encuentra el servicio
}

func (s *ServiceMetadata) ToMap() map[string]string {
	m := make(map[string]string)
	SetIfNotEmpty(m, "name", s.Name)
	SetIfNotEmpty(m, "product", s.Product)
	SetIfNotEmpty(m, "version", s.Version)
	SetIfNotEmpty(m, "extra_info", s.ExtraInfo)
	if s.Port > 0 {
		SetInt(m, "port", s.Port)
	}
	SetIfNotEmpty(m, "protocol", s.Protocol)
	SetIfNotEmpty(m, "state", s.State)
	SetIfNotEmpty(m, "banner", s.Banner)
	SetIfNotEmpty(m, "service_fp", s.ServiceFP)
	SetIfNotEmpty(m, "cpe", s.CPE)
	SetBool(m, "ssl_enabled", s.SSLEnabled)
	SetIfNotEmpty(m, "ssl_cert", s.SSLCert)
	SetBool(m, "has_vulns", s.HasVulns)
	if len(s.CVEList) > 0 {
		m["cve_list"] = StringSliceToCSV(s.CVEList)
	}
	SetIfNotEmpty(m, "risk_level", s.RiskLevel)
	if len(s.ScriptResults) > 0 {
		for k, v := range s.ScriptResults {
			m["script_"+k] = v
		}
	}
	SetIfNotEmpty(m, "detection_method", s.DetectionMethod)
	if s.Confidence > 0 {
		m["confidence"] = strconv.FormatFloat(s.Confidence, 'f', 2, 64)
	}
	SetIfNotEmpty(m, "scan_tool", s.ScanTool)
	SetIfNotEmpty(m, "parent_ip", s.ParentIP)
	return m
}

func (s *ServiceMetadata) FromMap(m map[string]string) error {
	s.Name = GetString(m, "name", "")
	s.Product = GetString(m, "product", "")
	s.Version = GetString(m, "version", "")
	s.ExtraInfo = GetString(m, "extra_info", "")
	s.Port = GetInt(m, "port", 0)
	s.Protocol = GetString(m, "protocol", "")
	s.State = GetString(m, "state", "")
	s.Banner = GetString(m, "banner", "")
	s.ServiceFP = GetString(m, "service_fp", "")
	s.CPE = GetString(m, "cpe", "")
	s.SSLEnabled = GetBool(m, "ssl_enabled", false)
	s.SSLCert = GetString(m, "ssl_cert", "")
	s.HasVulns = GetBool(m, "has_vulns", false)
	s.CVEList = CSVToStringSlice(GetString(m, "cve_list", ""))
	s.RiskLevel = GetString(m, "risk_level", "")

	// Parse script results
	s.ScriptResults = make(map[string]string)
	for k, v := range m {
		if strings.HasPrefix(k, "script_") {
			scriptName := strings.TrimPrefix(k, "script_")
			s.ScriptResults[scriptName] = v
		}
	}

	s.DetectionMethod = GetString(m, "detection_method", "")
	confStr := GetString(m, "confidence", "0")
	if conf, err := strconv.ParseFloat(confStr, 64); err == nil {
		s.Confidence = conf
	}
	s.ScanTool = GetString(m, "scan_tool", "")
	s.ParentIP = GetString(m, "parent_ip", "")
	return nil
}

func (s *ServiceMetadata) IsValid() bool { return s.Name != "" && s.Port > 0 }
func (s *ServiceMetadata) Type() string  { return "service" }

// NewServiceMetadata crea una instancia de ServiceMetadata con valores por defecto.
func NewServiceMetadata(name string, port int) *ServiceMetadata {
	return &ServiceMetadata{
		Name:          name,
		Port:          port,
		Protocol:      "tcp",
		State:         "open",
		ScriptResults: make(map[string]string),
		Confidence:    1.0,
	}
}
