package nm

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	nmBus       = "org.freedesktop.NetworkManager"
	nmPath      = "/org/freedesktop/NetworkManager"
	nmInterface = "org.freedesktop.NetworkManager"

	nmDeviceInterface   = "org.freedesktop.NetworkManager.Device"
	nmWirelessInterface = "org.freedesktop.NetworkManager.Device.Wireless"
	nmAPInterface       = "org.freedesktop.NetworkManager.AccessPoint"
	nmConnActive        = "org.freedesktop.NetworkManager.Connection.Active"
	nmSettings          = "org.freedesktop.NetworkManager.Settings"
	nmSettingsConn      = "org.freedesktop.NetworkManager.Settings.Connection"

	dbusProperties = "org.freedesktop.DBus.Properties"
)

// DeviceType represents NetworkManager device types.
type DeviceType uint32

const (
	DeviceTypeUnknown  DeviceType = 0
	DeviceTypeEthernet DeviceType = 1
	DeviceTypeWiFi     DeviceType = 2
	DeviceTypeBridge   DeviceType = 13
	DeviceTypeWireGuard DeviceType = 29
)

// DeviceState represents NetworkManager device states.
type DeviceState uint32

const (
	DeviceStateUnknown      DeviceState = 0
	DeviceStateUnmanaged    DeviceState = 10
	DeviceStateUnavailable  DeviceState = 20
	DeviceStateDisconnected DeviceState = 30
	DeviceStatePrepare      DeviceState = 40
	DeviceStateConfig       DeviceState = 50
	DeviceStateNeedAuth     DeviceState = 60
	DeviceStateIPConfig     DeviceState = 70
	DeviceStateIPCheck      DeviceState = 80
	DeviceStateSecondaries  DeviceState = 90
	DeviceStateActivated    DeviceState = 100
	DeviceStateDeactivating DeviceState = 110
	DeviceStateFailed       DeviceState = 120
)

// Device represents a network device.
type Device struct {
	Path      dbus.ObjectPath
	Name      string
	Type      DeviceType
	State     DeviceState
	HWAddr    string
	IP4Addr   string
	ActiveAP  dbus.ObjectPath
}

// AccessPoint represents a WiFi access point.
type AccessPoint struct {
	Path     dbus.ObjectPath
	SSID     string
	Strength uint8
	Freq     uint32
	Security string
	Active   bool
}

// Connection represents a saved connection profile.
type Connection struct {
	Path     dbus.ObjectPath
	ID       string
	Type     string
	AutoConn bool
}

// Client manages the D-Bus connection to NetworkManager.
type Client struct {
	conn *dbus.Conn
}

// NewClient creates a new NetworkManager D-Bus client.
func NewClient() (*Client, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to system bus: %w", err)
	}
	return &Client{conn: conn}, nil
}

// Close closes the D-Bus connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetDevices returns all network devices.
func (c *Client) GetDevices() ([]Device, error) {
	obj := c.conn.Object(nmBus, nmPath)
	var devicePaths []dbus.ObjectPath
	err := obj.Call(nmInterface+".GetDevices", 0).Store(&devicePaths)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	var devices []Device
	for _, path := range devicePaths {
		device, err := c.getDevice(path)
		if err != nil {
			continue
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func (c *Client) getDevice(path dbus.ObjectPath) (Device, error) {
	obj := c.conn.Object(nmBus, path)

	device := Device{Path: path}

	iface, err := c.getProperty(obj, nmDeviceInterface, "Interface")
	if err == nil {
		device.Name = iface.Value().(string)
	}

	devType, err := c.getProperty(obj, nmDeviceInterface, "DeviceType")
	if err == nil {
		device.Type = DeviceType(devType.Value().(uint32))
	}

	state, err := c.getProperty(obj, nmDeviceInterface, "State")
	if err == nil {
		device.State = DeviceState(state.Value().(uint32))
	}

	hwAddr, err := c.getProperty(obj, nmDeviceInterface, "HwAddress")
	if err == nil {
		device.HWAddr = hwAddr.Value().(string)
	}

	return device, nil
}

// GetAccessPoints returns visible WiFi access points for a wireless device.
func (c *Client) GetAccessPoints(devicePath dbus.ObjectPath) ([]AccessPoint, error) {
	obj := c.conn.Object(nmBus, devicePath)
	var apPaths []dbus.ObjectPath
	err := obj.Call(nmWirelessInterface+".GetAllAccessPoints", 0).Store(&apPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to get access points: %w", err)
	}

	// Get the active AP for this device
	activeAP := dbus.ObjectPath("/")
	prop, err := c.getProperty(obj, nmWirelessInterface, "ActiveAccessPoint")
	if err == nil {
		activeAP = prop.Value().(dbus.ObjectPath)
	}

	var accessPoints []AccessPoint
	for _, path := range apPaths {
		ap, err := c.getAccessPoint(path, activeAP)
		if err != nil {
			continue
		}
		if ap.SSID != "" {
			accessPoints = append(accessPoints, ap)
		}
	}
	return accessPoints, nil
}

func (c *Client) getAccessPoint(path dbus.ObjectPath, activeAP dbus.ObjectPath) (AccessPoint, error) {
	obj := c.conn.Object(nmBus, path)

	ap := AccessPoint{
		Path:   path,
		Active: path == activeAP,
	}

	ssid, err := c.getProperty(obj, nmAPInterface, "Ssid")
	if err == nil {
		ap.SSID = string(ssid.Value().([]byte))
	}

	strength, err := c.getProperty(obj, nmAPInterface, "Strength")
	if err == nil {
		ap.Strength = strength.Value().(uint8)
	}

	freq, err := c.getProperty(obj, nmAPInterface, "Frequency")
	if err == nil {
		ap.Freq = freq.Value().(uint32)
	}

	flags, err := c.getProperty(obj, nmAPInterface, "WpaFlags")
	if err == nil {
		wpaFlags := flags.Value().(uint32)
		rsnFlags := uint32(0)
		rsn, err := c.getProperty(obj, nmAPInterface, "RsnFlags")
		if err == nil {
			rsnFlags = rsn.Value().(uint32)
		}
		ap.Security = securityString(wpaFlags, rsnFlags)
	}

	return ap, nil
}

// RequestScan triggers a WiFi scan on the given wireless device.
func (c *Client) RequestScan(devicePath dbus.ObjectPath) error {
	obj := c.conn.Object(nmBus, devicePath)
	options := map[string]dbus.Variant{}
	return obj.Call(nmWirelessInterface+".RequestScan", 0, options).Err
}

// ActivateConnection connects to a saved connection profile.
func (c *Client) ActivateConnection(connPath, devicePath dbus.ObjectPath) error {
	obj := c.conn.Object(nmBus, nmPath)
	var activeConnPath dbus.ObjectPath
	return obj.Call(nmInterface+".ActivateConnection", 0, connPath, devicePath, dbus.ObjectPath("/")).Store(&activeConnPath)
}

// DeactivateConnection disconnects an active connection.
func (c *Client) DeactivateConnection(activeConnPath dbus.ObjectPath) error {
	obj := c.conn.Object(nmBus, nmPath)
	return obj.Call(nmInterface+".DeactivateConnection", 0, activeConnPath).Err
}

// GetActiveConnectionForDevice returns the active connection path for a device.
func (c *Client) GetActiveConnectionForDevice(devicePath dbus.ObjectPath) (dbus.ObjectPath, error) {
	obj := c.conn.Object(nmBus, devicePath)
	prop, err := c.getProperty(obj, nmDeviceInterface, "ActiveConnection")
	if err != nil {
		return "/", err
	}
	path := prop.Value().(dbus.ObjectPath)
	if path == "/" || path == "" {
		return "/", fmt.Errorf("no active connection on device")
	}
	return path, nil
}

// AddAndActivateConnection creates a new connection and activates it.
func (c *Client) AddAndActivateConnection(settings map[string]map[string]dbus.Variant, devicePath, apPath dbus.ObjectPath) error {
	obj := c.conn.Object(nmBus, nmPath)
	var connPath, activeConnPath dbus.ObjectPath
	return obj.Call(nmInterface+".AddAndActivateConnection", 0, settings, devicePath, apPath).Store(&connPath, &activeConnPath)
}

// GetSavedConnections returns all saved connection profiles.
func (c *Client) GetSavedConnections() ([]Connection, error) {
	obj := c.conn.Object(nmBus, "/org/freedesktop/NetworkManager/Settings")
	var connPaths []dbus.ObjectPath
	err := obj.Call(nmSettings+".ListConnections", 0).Store(&connPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to list connections: %w", err)
	}

	var connections []Connection
	for _, path := range connPaths {
		conn, err := c.getConnection(path)
		if err != nil {
			continue
		}
		connections = append(connections, conn)
	}
	return connections, nil
}

func (c *Client) getConnection(path dbus.ObjectPath) (Connection, error) {
	obj := c.conn.Object(nmBus, path)

	var settings map[string]map[string]dbus.Variant
	err := obj.Call(nmSettingsConn+".GetSettings", 0).Store(&settings)
	if err != nil {
		return Connection{}, err
	}

	conn := Connection{Path: path}

	if connSection, ok := settings["connection"]; ok {
		if id, ok := connSection["id"]; ok {
			conn.ID = id.Value().(string)
		}
		if connType, ok := connSection["type"]; ok {
			conn.Type = connType.Value().(string)
		}
		if autoConn, ok := connSection["autoconnect"]; ok {
			conn.AutoConn = autoConn.Value().(bool)
		} else {
			conn.AutoConn = true
		}
	}

	return conn, nil
}

// DeleteConnection removes a saved connection profile.
func (c *Client) DeleteConnection(connPath dbus.ObjectPath) error {
	obj := c.conn.Object(nmBus, connPath)
	return obj.Call(nmSettingsConn+".Delete", 0).Err
}

func (c *Client) getProperty(obj dbus.BusObject, iface, prop string) (dbus.Variant, error) {
	var result dbus.Variant
	err := obj.Call(dbusProperties+".Get", 0, iface, prop).Store(&result)
	return result, err
}

func securityString(wpaFlags, rsnFlags uint32) string {
	if rsnFlags != 0 {
		return "WPA2/WPA3"
	}
	if wpaFlags != 0 {
		return "WPA"
	}
	return "Open"
}

// DeviceTypeName returns a human-readable device type name.
func DeviceTypeName(t DeviceType) string {
	switch t {
	case DeviceTypeEthernet:
		return "Ethernet"
	case DeviceTypeWiFi:
		return "WiFi"
	case DeviceTypeBridge:
		return "Bridge"
	case DeviceTypeWireGuard:
		return "WireGuard"
	default:
		return "Unknown"
	}
}

// DeviceStateName returns a human-readable device state.
func DeviceStateName(s DeviceState) string {
	switch s {
	case DeviceStateActivated:
		return "Connected"
	case DeviceStateDisconnected:
		return "Disconnected"
	case DeviceStatePrepare, DeviceStateConfig, DeviceStateIPConfig, DeviceStateIPCheck:
		return "Connecting..."
	case DeviceStateNeedAuth:
		return "Auth required"
	case DeviceStateDeactivating:
		return "Disconnecting..."
	case DeviceStateFailed:
		return "Failed"
	case DeviceStateUnavailable:
		return "Unavailable"
	case DeviceStateUnmanaged:
		return "Unmanaged"
	default:
		return "Unknown"
	}
}
