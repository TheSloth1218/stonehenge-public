package telemetry

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"stonehenge-middleware/internal/arbitration"
	"stonehenge-middleware/internal/autocontrol"
	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/plc"
	"stonehenge-middleware/internal/safety"
	"stonehenge-middleware/internal/targetfeed"
)

const (
	Gun1AutoRegisterBase  uint16 = 0
	Gun1AutoRegisterCount uint16 = 96
)

const (
	functionReadHoldingRegisters   byte = 0x03
	functionReadInputRegisters     byte = 0x04
	functionWriteSingleCoil        byte = 0x05
	functionWriteSingleRegister    byte = 0x06
	functionWriteMultipleCoils     byte = 0x0f
	functionWriteMultipleRegisters byte = 0x10
	functionMaskWriteRegister      byte = 0x16
	functionReadWriteRegisters     byte = 0x17
)

const (
	exceptionIllegalFunction byte = 0x01
	exceptionIllegalAddress  byte = 0x02
	exceptionIllegalValue    byte = 0x03
	exceptionGatewayTarget   byte = 0x0b
)

type AutoSnapshotProvider interface {
	TelemetrySnapshot(plcStatus *plc.GunStatus, lastPLCUpdateAt *time.Time, plcStale bool) autocontrol.TelemetrySnapshot
}

type StatusReader interface {
	ReadStatus(ctx context.Context) (plc.GunStatus, error)
}

type FeedStatusProvider interface {
	Status() targetfeed.Status
}

type SafetyStatusProvider interface {
	Snapshot() safety.Snapshot
}

type ModbusStatus struct {
	Enabled         bool       `json:"modbus_enabled"`
	Addr            string     `json:"modbus_addr"`
	SchemaVersion   uint16     `json:"schema_version"`
	Healthy         bool       `json:"healthy"`
	LastPLCUpdateAt *time.Time `json:"last_plc_update_at,omitempty"`
	Stale           bool       `json:"stale"`
}

type ModbusServer struct {
	cfg        config.PLCConfig
	provider   AutoSnapshotProvider
	feed       FeedStatusProvider
	safety     SafetyStatusProvider
	reader     StatusReader
	logger     *slog.Logger
	listener   net.Listener
	addr       string
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	startOnce  sync.Once
	closeOnce  sync.Once
	statusMu   sync.RWMutex
	plcStatus  *plc.GunStatus
	lastUpdate *time.Time
	stale      bool
	lastHealth bool
}

func (s *ModbusServer) SetTargetFeed(feed FeedStatusProvider) {
	if s == nil {
		return
	}
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.feed = feed
}

func (s *ModbusServer) SetSafety(provider SafetyStatusProvider) {
	if s == nil {
		return
	}
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.safety = provider
}

func NewModbusServer(cfg config.PLCConfig, provider AutoSnapshotProvider, reader StatusReader, logger *slog.Logger) *ModbusServer {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ModbusServer{
		cfg:      cfg,
		provider: provider,
		reader:   reader,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		addr:     cfg.AutoTelemetryModbusAddr,
		stale:    true,
	}
}

func (s *ModbusServer) Start() error {
	if s == nil || !s.cfg.AutoTelemetryModbusEnabled {
		return nil
	}
	if s.provider == nil {
		return errors.New("auto telemetry snapshot provider is unavailable")
	}
	if s.reader == nil {
		return errors.New("auto telemetry plc status reader is unavailable")
	}
	var startErr error
	s.startOnce.Do(func() {
		listener, err := net.Listen("tcp", s.cfg.AutoTelemetryModbusAddr)
		if err != nil {
			startErr = err
			return
		}
		s.listener = listener
		s.addr = listener.Addr().String()
		s.logger.Info("gun 1 auto telemetry modbus server starting", "addr", s.addr, "unit_id", s.cfg.AutoTelemetryUnitID)
		s.wg.Add(2)
		go func() {
			defer s.wg.Done()
			s.pollPLC()
		}()
		go func() {
			defer s.wg.Done()
			s.serve(listener)
		}()
	})
	return startErr
}

func (s *ModbusServer) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		s.cancel()
		if s.listener != nil {
			_ = s.listener.Close()
		}
		s.wg.Wait()
		s.logger.Info("gun 1 auto telemetry modbus server stopped", "addr", s.addr)
	})
}

func (s *ModbusServer) Addr() string {
	if s == nil {
		return ""
	}
	return s.addr
}

func (s *ModbusServer) Status() ModbusStatus {
	snapshot := s.snapshot()
	return ModbusStatus{
		Enabled:         s.cfg.AutoTelemetryModbusEnabled,
		Addr:            s.cfg.AutoTelemetryModbusAddr,
		SchemaVersion:   autocontrol.TelemetrySchemaVersion,
		Healthy:         snapshot.Healthy,
		LastPLCUpdateAt: snapshot.LastPLCUpdateAt,
		Stale:           snapshot.PLCStale,
	}
}

func (s *ModbusServer) pollPLC() {
	interval := s.cfg.AutoTelemetryPollInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.pollPLCOnce()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.pollPLCOnce()
		}
	}
}

func (s *ModbusServer) pollPLCOnce() {
	ctx, cancel := context.WithTimeout(s.ctx, s.cfg.AutoTelemetryPollInterval)
	defer cancel()
	status, err := s.reader.ReadStatus(ctx)
	now := time.Now().UTC()
	s.statusMu.Lock()
	if err != nil {
		s.stale = true
		if s.lastHealth {
			s.logger.Warn("gun 1 auto telemetry plc status became stale", "error", err)
		}
		s.lastHealth = false
		s.statusMu.Unlock()
		return
	}
	s.plcStatus = &status
	s.lastUpdate = &now
	s.stale = false
	s.lastHealth = true
	s.statusMu.Unlock()
}

func (s *ModbusServer) updateStaleness() {
	staleAfter := s.cfg.AutoTelemetryStaleAfter
	if staleAfter <= 0 {
		staleAfter = 3 * time.Second
	}
	now := time.Now().UTC()
	s.statusMu.Lock()
	wasHealthy := s.lastHealth
	if s.lastUpdate == nil || now.Sub(*s.lastUpdate) > staleAfter {
		s.stale = true
		s.lastHealth = false
		if wasHealthy {
			s.logger.Warn("gun 1 auto telemetry plc status became stale", "last_update_at", s.lastUpdate)
		}
	} else {
		s.stale = false
		s.lastHealth = true
		if !wasHealthy {
			s.logger.Info("gun 1 auto telemetry plc status healthy", "last_update_at", s.lastUpdate)
		}
	}
	s.statusMu.Unlock()
}

func (s *ModbusServer) snapshot() autocontrol.TelemetrySnapshot {
	s.updateStaleness()
	s.statusMu.RLock()
	var status *plc.GunStatus
	var updated *time.Time
	if s.plcStatus != nil {
		copyStatus := *s.plcStatus
		status = &copyStatus
	}
	if s.lastUpdate != nil {
		copyUpdate := *s.lastUpdate
		updated = &copyUpdate
	}
	stale := s.stale
	s.statusMu.RUnlock()
	if s.provider == nil {
		return autocontrol.TelemetrySnapshot{SchemaVersion: autocontrol.TelemetrySchemaVersion, Healthy: false, PLCStale: true}
	}
	return s.provider.TelemetrySnapshot(status, updated, stale)
}

func (s *ModbusServer) serve(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			s.logger.Warn("gun 1 auto telemetry modbus accept failed", "error", err)
			continue
		}
		s.logger.Debug("gun 1 auto telemetry modbus client connected", "remote", conn.RemoteAddr().String())
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

func (s *ModbusServer) handleConn(conn net.Conn) {
	defer conn.Close()
	for {
		if s.ctx.Err() != nil {
			return
		}
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		header := make([]byte, 7)
		if _, err := io.ReadFull(conn, header); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				s.logger.Warn("gun 1 auto telemetry modbus malformed header", "error", err)
			}
			return
		}
		transactionID := binary.BigEndian.Uint16(header[0:2])
		protocolID := binary.BigEndian.Uint16(header[2:4])
		length := binary.BigEndian.Uint16(header[4:6])
		unitID := header[6]
		if protocolID != 0 || length < 2 || length > 254 {
			s.logger.Warn("gun 1 auto telemetry modbus malformed mbap", "protocol_id", protocolID, "length", length)
			return
		}
		pdu := make([]byte, int(length)-1)
		if _, err := io.ReadFull(conn, pdu); err != nil {
			s.logger.Warn("gun 1 auto telemetry modbus malformed pdu", "error", err)
			return
		}
		response := s.handlePDU(unitID, pdu)
		if response == nil {
			return
		}
		if err := writeModbusResponse(conn, transactionID, unitID, response); err != nil {
			s.logger.Warn("gun 1 auto telemetry modbus write failed", "error", err)
			return
		}
	}
}

func (s *ModbusServer) handlePDU(unitID byte, pdu []byte) []byte {
	if len(pdu) == 0 {
		s.logger.Warn("gun 1 auto telemetry modbus empty pdu")
		return nil
	}
	fn := pdu[0]
	if unitID != s.cfg.AutoTelemetryUnitID {
		return exceptionResponse(fn, exceptionGatewayTarget)
	}
	switch fn {
	case functionReadHoldingRegisters, functionReadInputRegisters:
		if len(pdu) != 5 {
			return exceptionResponse(fn, exceptionIllegalValue)
		}
		address := binary.BigEndian.Uint16(pdu[1:3])
		quantity := binary.BigEndian.Uint16(pdu[3:5])
		return s.readRegisters(fn, address, quantity)
	case functionWriteSingleCoil, functionWriteSingleRegister, functionWriteMultipleCoils, functionWriteMultipleRegisters, functionMaskWriteRegister, functionReadWriteRegisters:
		s.logger.Warn("gun 1 auto telemetry rejected modbus write request", "function", fmt.Sprintf("0x%02x", fn))
		return exceptionResponse(fn, exceptionIllegalFunction)
	default:
		return exceptionResponse(fn, exceptionIllegalFunction)
	}
}

func (s *ModbusServer) readRegisters(fn byte, address, quantity uint16) []byte {
	if quantity == 0 || quantity > 125 {
		return exceptionResponse(fn, exceptionIllegalValue)
	}
	if address < Gun1AutoRegisterBase || address+quantity > Gun1AutoRegisterBase+Gun1AutoRegisterCount || address+quantity < address {
		return exceptionResponse(fn, exceptionIllegalAddress)
	}
	now := time.Now().UTC()
	snapshot := s.snapshot()
	registers := EncodeGun1AutoRegisters(snapshot, now)
	if feed := s.feedStatus(); feed != nil {
		registers = EncodeGun1AutoRegistersWithFeed(snapshot, *feed, now)
	}
	if safetyStatus := s.safetyStatus(); safetyStatus != nil {
		registers = EncodeGun1AutoRegistersWithFeedAndSafety(snapshot, s.feedStatusValue(), *safetyStatus, now)
	}
	byteCount := int(quantity) * 2
	response := make([]byte, 2+byteCount)
	response[0] = fn
	response[1] = byte(byteCount)
	for i := uint16(0); i < quantity; i++ {
		binary.BigEndian.PutUint16(response[2+int(i)*2:], registers[int(address-Gun1AutoRegisterBase+i)])
	}
	return response
}

func (s *ModbusServer) feedStatus() *targetfeed.Status {
	s.statusMu.RLock()
	feed := s.feed
	s.statusMu.RUnlock()
	if feed == nil {
		return nil
	}
	status := feed.Status()
	return &status
}

func (s *ModbusServer) feedStatusValue() targetfeed.Status {
	if feed := s.feedStatus(); feed != nil {
		return *feed
	}
	return targetfeed.Status{}
}

func (s *ModbusServer) safetyStatus() *safety.Snapshot {
	s.statusMu.RLock()
	provider := s.safety
	s.statusMu.RUnlock()
	if provider == nil {
		return nil
	}
	status := provider.Snapshot()
	return &status
}

func writeModbusResponse(w io.Writer, transactionID uint16, unitID byte, pdu []byte) error {
	header := make([]byte, 7)
	binary.BigEndian.PutUint16(header[0:2], transactionID)
	binary.BigEndian.PutUint16(header[2:4], 0)
	binary.BigEndian.PutUint16(header[4:6], uint16(len(pdu)+1))
	header[6] = unitID
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(pdu)
	return err
}

func exceptionResponse(fn, code byte) []byte {
	return []byte{fn | 0x80, code}
}

func EncodeGun1AutoRegisters(snapshot autocontrol.TelemetrySnapshot, now time.Time) [Gun1AutoRegisterCount]uint16 {
	return EncodeGun1AutoRegistersWithFeed(snapshot, targetfeed.Status{}, now)
}

func EncodeGun1AutoRegistersWithFeed(snapshot autocontrol.TelemetrySnapshot, feed targetfeed.Status, now time.Time) [Gun1AutoRegisterCount]uint16 {
	return EncodeGun1AutoRegistersWithFeedAndSafety(snapshot, feed, safety.Snapshot{}, now)
}

func EncodeGun1AutoRegistersWithFeedAndSafety(snapshot autocontrol.TelemetrySnapshot, feed targetfeed.Status, safetyStatus safety.Snapshot, now time.Time) [Gun1AutoRegisterCount]uint16 {
	var registers [Gun1AutoRegisterCount]uint16
	registers[0] = snapshot.SchemaVersion
	registers[1] = boolWord(snapshot.Healthy)
	registers[2] = boolWord(snapshot.Enabled)
	registers[3] = snapshot.State.Code()
	putUint32(registers[:], 4, snapshot.QueuedTargetID)
	putUint32(registers[:], 6, snapshot.ActiveTargetID)
	putUint32(registers[:], 8, snapshot.ActiveSequence)
	registers[10] = autocontrol.ResultCode(snapshot.LastResult)
	registers[11] = snapshot.LastErrorCode
	if snapshot.PLCStale && registers[11] == autocontrol.ErrorCodeNone {
		registers[11] = autocontrol.ErrorCodePLCUnreachable
	}
	if !snapshot.LastStateChangedAt.IsZero() {
		age := uint32(now.Sub(snapshot.LastStateChangedAt).Seconds())
		putUint32Value(registers[:], 12, age)
	}
	registers[14] = uint16(snapshot.TargetAzimuthTenths)
	registers[15] = uint16(snapshot.TargetElevationTenths)
	registers[16] = snapshot.TargetPriority
	if snapshot.PLC != nil {
		status := snapshot.PLC
		registers[17] = status.State
		registers[18] = boolWord(status.Ready)
		registers[19] = boolWord(status.FaultActive)
		registers[20] = status.FaultCode
		putUint32Value(registers[:], 21, status.ShotSequence)
		putUint32Value(registers[:], 23, status.AcceptedSequence)
		registers[25] = uint16(status.CurrentAzimuthTenths)
		registers[26] = uint16(status.CurrentElevationTenths)
		registers[27] = boolWord(status.Settling)
		registers[28] = boolWord(status.Firing)
		registers[29] = boolWord(status.Lockout)
		registers[30] = boolWord(status.OnTarget)
	}
	if snapshot.Arbitration != nil {
		status := snapshot.Arbitration
		registers[32] = status.Mode.Code()
		registers[33] = status.ActiveOperation.Code()
		registers[34] = boolWord(status.FaultResetActive)
		registers[35] = boolWord(status.Mode == arbitration.ModeAutoSuspended)
		registers[36] = uint16(status.LastArbitrationErrorCode)
		registers[37] = arbitration.FaultResetResult(status.LastFaultResetResult).Code()
		putUint32(registers[:], 38, status.LastFaultResetSequence)
		if status.LastFaultCodeBefore != nil {
			registers[40] = *status.LastFaultCodeBefore
		}
		if status.LastFaultCodeAfter != nil {
			registers[41] = *status.LastFaultCodeAfter
		}
		if !status.LastChangedAt.IsZero() {
			putUint32Value(registers[:], 42, uint32(now.Sub(status.LastChangedAt).Seconds()))
		}
		if status.ResetStateChangedAt != nil {
			putUint32Value(registers[:], 44, uint32(now.Sub(*status.ResetStateChangedAt).Seconds()))
		}
	}
	registers[48] = boolWord(feed.Configured)
	registers[49] = boolWord(feed.Healthy)
	registers[50] = boolWord(feed.Receiving)
	registers[51] = uint16(feed.PendingCount)
	putUint32(registers[:], 52, feed.ActiveTargetID)
	putUint32Value(registers[:], 54, feed.ReceivedCount)
	putUint32Value(registers[:], 56, feed.AcceptedCount)
	putUint32Value(registers[:], 58, feed.DuplicateCount)
	putUint32Value(registers[:], 60, feed.RejectedCount)
	registers[62] = feed.LastErrorCode
	registers[64] = boolWord(safetyStatus.Configured)
	registers[65] = boolWord(safetyStatus.EnforcementEnabled)
	registers[66] = boolWord(safetyStatus.Connected)
	registers[67] = boolWord(safetyStatus.Fresh)
	registers[68] = safetyStatus.SchemaVersion
	registers[69] = safetyStatus.HeartbeatCounter
	registers[70] = uint16(safetyStatus.State)
	registers[71] = boolWord(safetyStatus.Ready)
	registers[72] = boolWord(safetyStatus.GlobalFirePermissive)
	registers[73] = boolWord(safetyStatus.GlobalInhibitActive)
	registers[74] = uint16(safetyStatus.GlobalInhibitReason)
	registers[75] = boolWord(safetyStatus.FaultActive)
	registers[76] = uint16(safetyStatus.FaultCode)
	registers[77] = boolWord(safetyStatus.ResetRequired)
	registers[78] = boolWord(safetyStatus.EmergencyStopActive)
	registers[79] = boolWord(safetyStatus.MaintenanceModeActive)
	for _, gun := range safetyStatus.Guns {
		if gun.GunID != 1 {
			continue
		}
		registers[80] = boolWord(gun.FirePermissive)
		registers[81] = boolWord(gun.InhibitActive)
		registers[82] = uint16(gun.InhibitReason)
		registers[83] = boolWord(gun.EnabledStatus)
		registers[84] = boolWord(gun.MaintenanceInhibitStatus)
		registers[85] = boolWord(gun.ExternalInhibitStatus)
		break
	}
	putUint32Value(registers[:], 86, safetyStatus.AgeSeconds)
	registers[88] = safetyStatus.LastErrorCode
	registers[89] = boolWord(safetyStatus.SchemaSupported)
	return registers
}

func boolWord(value bool) uint16 {
	if value {
		return 1
	}
	return 0
}

func putUint32(registers []uint16, offset int, value *uint32) {
	if value == nil {
		return
	}
	putUint32Value(registers, offset, *value)
}

func putUint32Value(registers []uint16, offset int, value uint32) {
	registers[offset] = uint16(value & 0xffff)
	registers[offset+1] = uint16(value >> 16)
}
