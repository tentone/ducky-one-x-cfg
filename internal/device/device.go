// Package device provides the native cross-platform HID connection.
package device

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/joseferrao/ducky-drv/internal/protocol"
	"github.com/sstallion/go-hid"
)

const (
	maxReportBytes = 256
	ioTimeout      = 5 * time.Second
	commandSpacing = 20 * time.Millisecond
)

type Descriptor struct {
	Path         string
	ProductID    uint16
	Product      string
	Manufacturer string
	Serial       string
	UsagePage    uint16
	Interface    int
}

func (d Descriptor) DisplayName() string {
	name := strings.TrimSpace(d.Product)
	if name == "" {
		name = "Ducky One X"
	}
	if d.Serial != "" {
		name += " · " + d.Serial
	}
	return name
}

type Manager struct {
	mu          sync.Mutex
	initialized bool
	session     *Session
}

func NewManager() (*Manager, error) {
	if err := hid.Init(); err != nil {
		return nil, fmt.Errorf("initialize HID: %w", err)
	}
	return &Manager{initialized: true}, nil
}

func (m *Manager) List() ([]Descriptor, error) {
	devices := make([]Descriptor, 0, 2)
	err := hid.Enumerate(protocol.VendorID, hid.ProductIDAny, func(info *hid.DeviceInfo) error {
		if info.UsagePage != protocol.ConfigUsagePage {
			return nil
		}
		if info.ProductStr != "" && !strings.Contains(strings.ToLower(info.ProductStr), "ducky one x") {
			return nil
		}
		devices = append(devices, Descriptor{
			Path: info.Path, ProductID: info.ProductID, Product: info.ProductStr,
			Manufacturer: info.MfrStr, Serial: info.SerialNbr, UsagePage: info.UsagePage,
			Interface: info.InterfaceNbr,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("enumerate HID devices: %w", err)
	}
	return devices, nil
}

func (m *Manager) Connect(path string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session != nil {
		_ = m.session.close()
		m.session = nil
	}
	dev, err := hid.OpenPath(path)
	if err != nil {
		return nil, fmt.Errorf("open keyboard: %w", err)
	}
	m.session = &Session{device: dev}
	return m.session, nil
}

func (m *Manager) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session == nil {
		return nil
	}
	err := m.session.close()
	m.session = nil
	return err
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var closeErr error
	if m.session != nil {
		closeErr = m.session.close()
		m.session = nil
	}
	if m.initialized {
		if err := hid.Exit(); err != nil && closeErr == nil {
			closeErr = err
		}
		m.initialized = false
	}
	return closeErr
}

type Session struct {
	mu           sync.Mutex
	device       *hid.Device
	closed       bool
	lastExchange time.Time
}

// Send writes a command that the keyboard may apply without acknowledging.
// Any late acknowledgement is harmless: the next Exchange filters reports by
// command before returning them to the protocol layer.
func (s *Session) Send(ctx context.Context, report []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.device == nil {
		return errors.New("keyboard is disconnected")
	}
	if len(report)+1 > maxReportBytes {
		return fmt.Errorf("output report is too large: %d bytes", len(report))
	}
	if wait := commandSpacing - time.Since(s.lastExchange); wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	out := make([]byte, len(report)+1)
	copy(out[1:], report)
	if _, err := s.device.Write(out); err != nil {
		return fmt.Errorf("write output report: %w", err)
	}
	s.lastExchange = time.Now()
	return nil
}

func (s *Session) Exchange(ctx context.Context, report []byte, expectedCommand byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.device == nil {
		return nil, errors.New("keyboard is disconnected")
	}
	if len(report)+1 > maxReportBytes {
		return nil, fmt.Errorf("output report is too large: %d bytes", len(report))
	}
	if wait := commandSpacing - time.Since(s.lastExchange); wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	defer func() { s.lastExchange = time.Now() }()
	// HIDAPI pads short reports to the descriptor size on platforms that
	// require it (notably Windows). Passing only the meaningful bytes also
	// avoids exceeding a device whose report is smaller than our read buffer.
	out := make([]byte, len(report)+1)
	out[0] = 0 // HID report ID, matching WebHID sendReport(0, ...).
	copy(out[1:], report)
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := s.device.Write(out); err != nil {
			return nil, fmt.Errorf("write output report: %w", err)
		}

		deadline := time.Now().Add(ioTimeout)
		if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
			deadline = contextDeadline
		}
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			remaining := time.Until(deadline)
			if remaining <= 0 {
				break
			}
			in := make([]byte, maxReportBytes)
			n, err := s.device.ReadWithTimeout(in, remaining)
			if err != nil {
				if errors.Is(err, hid.ErrTimeout) {
					break
				}
				return nil, fmt.Errorf("read input report: %w", err)
			}
			packet := normalizeInput(in[:n])
			if len(packet) < 3 {
				continue
			}
			if packet[0] != 0x66 || packet[2] != expectedCommand {
				// Profile-change and matrix-test reports can arrive asynchronously.
				continue
			}
			return append([]byte(nil), packet...), nil
		}
		if attempt == 0 {
			timer := time.NewTimer(commandSpacing)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil, fmt.Errorf("waiting for command 0x%02x after retry: %w", expectedCommand, hid.ErrTimeout)
}

func normalizeInput(report []byte) []byte {
	for len(report) > 0 && report[0] == 0 {
		report = report[1:]
	}
	if len(report) > 0 && report[0] == 0x66 {
		return report
	}
	return report
}

func (s *Session) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.device == nil {
		return nil
	}
	err := s.device.Close()
	s.device = nil
	return err
}
