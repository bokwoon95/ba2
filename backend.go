package main

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/playwright-community/playwright-go"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func init() {
	application.RegisterEvent[UpdateEvent]("backend:update")
}

type Backend struct {
	App                       *application.App
	PlaywrightDriver          *playwright.PlaywrightDriver
	PlaywrightDriverDirectory string
	Windows                   map[string]*application.WebviewWindow
	WindowsMutex              sync.RWMutex
}

type UpdateEvent struct {
	EventID  string `json:"eventID"`
	Category string `json:"category"`
	Message  string `json:"message"`
}

var _ http.Handler = (*Backend)(nil)

func (backend *Backend) Hello() string { return "hello" }

func (backend *Backend) Dialog(options MessageDialogOptions) {
	var messageDialog *application.MessageDialog
	switch options.DialogType {
	case "Info":
		messageDialog = backend.App.Dialog.Info()
	case "Question":
		messageDialog = backend.App.Dialog.Question()
	case "Warning":
		messageDialog = backend.App.Dialog.Warning()
	default:
		messageDialog = backend.App.Dialog.Error()
	}
	messageDialog.SetTitle(options.Title)
	messageDialog.SetMessage(options.Message)
	messageDialog.Show()
}

func (backend *Backend) CreateWindow(options WebviewWindowOptions) error {
	name := options.Name
	backend.WindowsMutex.Lock()
	defer backend.WindowsMutex.Unlock()
	window, ok := backend.Windows[name]
	if !ok {
		window = backend.App.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:  options.Name,
			Title: options.Title,
			URL:   options.URL,
		})
		backend.Windows[name] = window
	} else {
		window.SetTitle(options.Title)
		window.SetURL(options.URL)
		window.Show()
	}
	window.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		backend.WindowsMutex.Lock()
		defer backend.WindowsMutex.Unlock()
		backend.App.Event.EmitEvent(&application.CustomEvent{
			Name:   "backend:windowclosed",
			Sender: name,
		})
		delete(backend.Windows, name)
	})
	return nil
}

func (backend *Backend) EnableWindow(name string, enabled bool) error {
	backend.WindowsMutex.Lock()
	defer backend.WindowsMutex.Unlock()
	window, ok := backend.Windows[name]
	if !ok {
		return fmt.Errorf("no such window: %s", name)
	}
	window.SetEnabled(enabled)
	return nil
}

func (backend *Backend) ShowWindow(name string, show bool) error {
	backend.WindowsMutex.Lock()
	defer backend.WindowsMutex.Unlock()
	window, ok := backend.Windows[name]
	if !ok {
		return fmt.Errorf("no such window: %s", name)
	}
	if show {
		window.Show()
	} else {
		window.Hide()
	}
	return nil
}

func (backend *Backend) FocusWindow(name string) error {
	backend.WindowsMutex.Lock()
	defer backend.WindowsMutex.Unlock()
	window, ok := backend.Windows[name]
	if !ok {
		return fmt.Errorf("no such window: %s", name)
	}
	window.Focus()
	return nil
}

// HumanReadableFileSize returns a human readable file size of an int64 size in
// bytes.
func HumanReadableFileSize(size int64) string {
	// https://yourbasic.org/golang/formatting-byte-size-to-human-readable-format/
	if size < 0 {
		return ""
	}
	const unit = 1000
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "kMGTPE"[exp])
}

type WebviewWindowOptions struct {
	// Name is a unique identifier that can be given to a window.
	Name string

	// Title is the title of the window.
	Title string

	// Width is the starting width of the window.
	Width int

	// Height is the starting height of the window.
	Height int

	// AlwaysOnTop will make the window float above other windows.
	AlwaysOnTop bool

	// URL is the URL to load in the window.
	URL string

	// DisableResize will disable the ability to resize the window.
	DisableResize bool

	// Frameless will remove the window frame.
	Frameless bool

	// MinWidth is the minimum width of the window.
	MinWidth int

	// MinHeight is the minimum height of the window.
	MinHeight int

	// MaxWidth is the maximum width of the window.
	MaxWidth int

	// MaxHeight is the maximum height of the window.
	MaxHeight int

	// StartState indicates the state of the window when it is first shown.
	// Default: WindowStateNormal
	StartState string // WindowStateNormal|WindowStateMinimised|WindowStateMaximised|WindowStateFullscreen

	// BackgroundType is the type of background to use for the window.
	// Default: BackgroundTypeSolid
	BackgroundType string // BackgroundTypeSolid|BackgroundTypeTransparent|BackgroundTypeTranslucent

	// BackgroundColour is the colour to use for the window background.
	BackgroundColour [4]uint8 // R G B A

	// HTML is the HTML to load in the window.
	HTML string

	// JS is the JavaScript to load in the window.
	JS string

	// CSS is the CSS to load in the window.
	CSS string

	// Initial Position
	InitialPosition string // WindowCentered|WindowXY

	// X is the starting X position of the window.
	X int

	// Y is the starting Y position of the window.
	Y int

	// Hidden will hide the window when it is first created.
	Hidden bool

	// Zoom is the zoom level of the window.
	Zoom float64

	// ZoomControlEnabled will enable the zoom control.
	ZoomControlEnabled bool

	// EnableFileDrop enables drag and drop of files onto the window.
	// When enabled, files dragged from the OS onto elements with the
	// `data-file-drop-target` attribute will trigger a FilesDropped event.
	EnableFileDrop bool

	// OpenInspectorOnStartup will open the inspector when the window is first shown.
	OpenInspectorOnStartup bool

	// Mac options
	Mac application.MacWindow

	// Windows options
	Windows application.WindowsWindow

	// Linux options
	Linux application.LinuxWindow

	// Toolbar button states
	MinimiseButtonState string // ButtonEnabled|ButtonDisabled|ButtonHidden
	MaximiseButtonState string // ButtonEnabled|ButtonDisabled|ButtonHidden
	CloseButtonState    string // ButtonEnabled|ButtonDisabled|ButtonHidden

	// If true, the window's devtools will be available (default true in builds without the `production` build tag)
	DevToolsEnabled bool

	// If true, the window's default context menu will be disabled (default false)
	DefaultContextMenuDisabled bool

	// KeyBindings is a map of key bindings to functions
	// KeyBindings map[string]func(window application.Window)

	// IgnoreMouseEvents will ignore mouse events in the window (Windows + Mac only)
	IgnoreMouseEvents bool

	// ContentProtectionEnabled specifies whether content protection is enabled, preventing screen capture and recording.
	// Effective on Windows and macOS only; no-op on Linux.
	// Best-effort protection with platform-specific caveats (see docs).
	ContentProtectionEnabled bool

	// HideOnFocusLost will hide the window when it loses focus.
	// Useful for popup/transient windows like systray attached windows.
	// On Linux with focus-follows-mouse WMs (Hyprland, Sway, i3), this is automatically disabled
	// as it would cause the window to hide immediately when the mouse moves away.
	HideOnFocusLost bool

	// HideOnEscape will hide the window when the Escape key is pressed.
	// Useful for popup/transient windows that should dismiss on Escape.
	HideOnEscape bool

	// UseApplicationMenu indicates this window should use the application menu
	// set via app.Menu.Set() instead of requiring a window-specific menu.
	// On macOS this has no effect as the application menu is always global.
	// On Windows/Linux, if true and no explicit window menu is set, the window
	// will use the application menu. Defaults to false for backwards compatibility.
	UseApplicationMenu bool
}

type MessageDialogOptions struct {
	DialogType string // Info|Question|Warning|Error
	Title      string
	Message    string
}
