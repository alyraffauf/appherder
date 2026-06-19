//go:build gtk

package main

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/alyraffauf/appherder/internal/appherder"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

type mainWindow struct {
	*adw.ApplicationWindow

	app             appherder.App
	split           *adw.NavigationSplitView
	apps            []appherder.AppInfo
	updateChecks    map[string]appherder.UpgradeCheck
	checkingUpdates bool
	checkedUpdates  bool
	currentKey      string
	list            *gtk.ListBox
	installBtn      *gtk.MenuButton
	menuBtn         *gtk.MenuButton
	checkAction     *gio.SimpleAction
	updateAllAction *gio.SimpleAction
	busy            int
}

func newMainWindow(gtkApp *adw.Application, app appherder.App) *mainWindow {
	win := &mainWindow{
		ApplicationWindow: adw.NewApplicationWindow(&gtkApp.Application),
		app:               app,
		split:             adw.NewNavigationSplitView(),
		list:              gtk.NewListBox(),
		installBtn:        gtk.NewMenuButton(),
		menuBtn:           gtk.NewMenuButton(),
	}

	win.SetTitle("AppHerder")
	win.SetDefaultSize(760, 520)
	win.installActions()

	win.split.SetShowContent(true)
	win.split.SetMinSidebarWidth(260)
	win.split.SetMaxSidebarWidth(300)
	win.split.SetSidebar(adw.NewNavigationPage(win.sidebarView(), "AppHerder"))
	win.split.SetContent(adw.NewNavigationPage(emptyDetailsPage(), "AppHerder"))
	win.installBreakpoints()
	win.installDropTarget()

	win.SetContent(win.split)

	win.loadApps()
	win.checkUpdates(false)
	return win
}

func (w *mainWindow) installBreakpoints() {
	condition := adw.NewBreakpointConditionLength(adw.BreakpointConditionMaxWidth, 600, adw.LengthUnitSp)
	breakpoint := adw.NewBreakpoint(condition)
	breakpoint.AddSetterDirect(w.split.Object, "collapsed", glib.NewValue(true))
	breakpoint.AddSetterDirect(w.split.Object, "show-content", glib.NewValue(false))
	w.AddBreakpoint(breakpoint)
}

func (w *mainWindow) installDropTarget() {
	target := gtk.NewDropTarget(gdk.GTypeFileList, gdk.ActionCopy)
	target.ConnectDrop(func(value *glib.Value, x, y float64) bool {
		fileList, ok := value.GoValue().(*gdk.FileList)
		if !ok || fileList == nil {
			return false
		}
		return w.installDroppedFiles(fileList.Files())
	})
	w.split.AddController(target)
}

func (w *mainWindow) sidebarView() *adw.ToolbarView {
	title := adw.NewWindowTitle("AppHerder", "")
	header := adw.NewHeaderBar()
	header.SetShowEndTitleButtons(false)
	header.SetTitleWidget(title)
	header.PackStart(w.installBtn)
	header.PackEnd(w.menuBtn)

	w.list.AddCSSClass("navigation-sidebar")
	w.list.SetSelectionMode(gtk.SelectionSingle)
	w.list.ConnectRowActivated(func(row *gtk.ListBoxRow) {
		w.activateAppRow(row)
	})
	w.list.ConnectRowSelected(func(row *gtk.ListBoxRow) {
		w.selectAppRow(row)
	})

	scroller := gtk.NewScrolledWindow()
	scroller.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroller.SetChild(w.list)
	scroller.SetVExpand(true)

	toolbar := adw.NewToolbarView()
	toolbar.AddTopBar(header)
	toolbar.SetContent(scroller)
	return toolbar
}

func emptyDetailsPage() *adw.ToolbarView {
	header := adw.NewHeaderBar()

	empty := adw.NewStatusPage()
	empty.SetIconName("application-x-executable-symbolic")
	empty.SetTitle("No AppImage Selected")
	empty.SetDescription("Select an installed AppImage to view details.")

	toolbar := adw.NewToolbarView()
	toolbar.AddTopBar(header)
	toolbar.SetContent(empty)
	return toolbar
}

func (w *mainWindow) installActions() {
	w.installBtn.SetIconName("list-add-symbolic")
	w.installBtn.SetTooltipText("Install AppImage")
	installMenu := gio.NewMenu()
	installMenu.Append("Install from File", "win.install-file")
	installMenu.Append("Install from URL", "win.install-url")
	w.installBtn.SetMenuModel(installMenu)

	w.menuBtn.SetIconName("open-menu-symbolic")
	w.menuBtn.SetTooltipText("Main Menu")

	menu := gio.NewMenu()
	updateSection := gio.NewMenu()
	updateSection.Append("Check for Updates", "win.check-upgrades")
	updateSection.Append("Update All", "win.apply-upgrades")
	menu.AppendSection("", updateSection)
	aboutSection := gio.NewMenu()
	aboutSection.Append("About AppHerder", "win.about")
	menu.AppendSection("", aboutSection)
	w.menuBtn.SetMenuModel(menu)

	w.checkAction = w.addAction("check-upgrades", func() { w.checkUpdates(true) })
	w.updateAllAction = w.addAction("apply-upgrades", w.applyUpgrades)
	w.addAction("install-file", w.promptInstallFile)
	w.addAction("install-url", w.promptInstallURL)
	w.addAction("about", w.showAbout)
	w.updateActionSensitivity()
}

func (w *mainWindow) addAction(name string, activate func()) *gio.SimpleAction {
	action := gio.NewSimpleAction(name, nil)
	action.ConnectActivate(func(parameter *glib.Variant) {
		activate()
	})
	w.AddAction(action)
	return action
}

func (w *mainWindow) showAbout() {
	about := adw.NewAboutDialog()
	about.SetApplicationName("AppHerder")
	about.SetApplicationIcon("application-x-executable-symbolic")
	about.SetVersion(version)
	about.SetDeveloperName("Aly Raffauf")
	about.SetDevelopers([]string{"Aly Raffauf"})
	about.SetComments("Manage AppImages installed in your home directory.")
	about.SetWebsite("https://github.com/alyraffauf/appherder")
	about.SetIssueURL("https://github.com/alyraffauf/appherder/issues")
	about.SetLicenseType(gtk.LicenseGPL30)
	about.Present(w)
}

func (w *mainWindow) loadApps() {
	w.run("Refreshing app list", func() (string, error) {
		infos, err := w.app.List()
		if err != nil {
			return "", err
		}
		w.idle(func() {
			w.renderApps(infos)
		})
		return "", nil
	})
}

func (w *mainWindow) renderApps(infos []appherder.AppInfo) {
	w.apps = infos
	w.list.RemoveAll()
	if len(infos) == 0 {
		w.apps = nil
		w.currentKey = ""
		w.split.SetContent(adw.NewNavigationPage(emptyDetailsPage(), "AppHerder"))
		return
	}
	selected := w.currentKey
	for i, info := range infos {
		w.list.Append(w.appListRow(info))
		if selected == "" && i == 0 {
			w.showDetails(info, false)
		}
	}
	rowIndex := 0
	if selected != "" {
		for i, info := range infos {
			if appKey(info) == selected {
				rowIndex = i
				break
			}
		}
	}
	if row := w.list.RowAtIndex(rowIndex); row != nil {
		w.list.SelectRow(row)
	}
}

func (w *mainWindow) activateAppRow(row *gtk.ListBoxRow) {
	w.showDetailsForRow(row, true)
}

func (w *mainWindow) selectAppRow(row *gtk.ListBoxRow) {
	w.showDetailsForRow(row, false)
}

func (w *mainWindow) showDetailsForRow(row *gtk.ListBoxRow, reveal bool) {
	if row == nil {
		return
	}
	index := row.Index()
	if index < 0 || index >= len(w.apps) {
		return
	}
	w.showDetails(w.apps[index], reveal)
}

func (w *mainWindow) appListRow(info appherder.AppInfo) *gtk.ListBoxRow {
	row := gtk.NewListBoxRow()
	row.SetActivatable(true)
	row.SetSelectable(true)

	box := gtk.NewBox(gtk.OrientationHorizontal, 12)
	box.SetMarginTop(8)
	box.SetMarginBottom(8)
	box.SetMarginStart(12)
	box.SetMarginEnd(12)

	icon := appIcon(info, 36)
	icon.SetVAlign(gtk.AlignCenter)
	box.Append(icon)

	name := gtk.NewLabel(info.Name)
	name.SetXAlign(0)
	name.SetEllipsize(pango.EllipsizeEnd)
	name.SetLines(1)
	name.SetHExpand(true)
	name.SetVAlign(gtk.AlignCenter)
	box.Append(name)

	if w.updateAvailable(info) {
		indicator := gtk.NewLabel("Update")
		indicator.AddCSSClass("caption")
		indicator.AddCSSClass("dim-label")
		indicator.SetVAlign(gtk.AlignCenter)
		indicator.SetMarginStart(6)
		indicator.SetTooltipText("Update Available")
		box.Append(indicator)
	}

	row.SetChild(box)
	return row
}

func appSummary(info appherder.AppInfo) string {
	if info.Filename != "" {
		return info.Filename
	}
	return info.AppID
}

func (w *mainWindow) showDetails(info appherder.AppInfo, reveal bool) {
	w.currentKey = appKey(info)
	w.split.SetContent(adw.NewNavigationPage(w.appDetailsView(info), info.Name))
	if reveal {
		w.split.SetShowContent(true)
	}
}

func (w *mainWindow) appDetailsView(info appherder.AppInfo) *adw.ToolbarView {
	header := adw.NewHeaderBar()

	hero := gtk.NewBox(gtk.OrientationVertical, 8)
	hero.SetMarginTop(24)
	hero.SetMarginBottom(20)
	hero.SetMarginStart(24)
	hero.SetMarginEnd(24)
	hero.SetHAlign(gtk.AlignCenter)
	hero.Append(appIcon(info, 88))

	summary := gtk.NewBox(gtk.OrientationVertical, 4)
	summary.SetHAlign(gtk.AlignCenter)

	name := gtk.NewLabel(info.Name)
	name.AddCSSClass("title-1")
	name.SetEllipsize(pango.EllipsizeEnd)
	name.SetMaxWidthChars(28)
	name.SetLines(1)

	subtitle := gtk.NewLabel(appSummary(info))
	subtitle.SetWrap(true)
	subtitle.AddCSSClass("dim-label")
	subtitle.SetEllipsize(pango.EllipsizeEnd)
	subtitle.SetMaxWidthChars(34)
	subtitle.SetLines(1)

	summary.Append(name)
	summary.Append(subtitle)

	actions := gtk.NewBox(gtk.OrientationHorizontal, 8)
	actions.SetMarginTop(10)

	launch := gtk.NewButtonWithLabel("Launch")
	launch.AddCSSClass("pill")
	launch.AddCSSClass("suggested-action")
	launch.ConnectClicked(func() { w.launchApp(info) })
	actions.Append(launch)

	if check, ok := w.updateCheck(info); ok && check.Available {
		update := gtk.NewButtonWithLabel("Update")
		update.AddCSSClass("pill")
		update.ConnectClicked(func() { w.updateApp(info) })
		actions.Append(update)
	}

	remove := gtk.NewButtonWithLabel("Remove")
	remove.AddCSSClass("pill")
	remove.AddCSSClass("destructive-action")
	remove.ConnectClicked(func() { w.confirmUninstall(info) })
	actions.Append(remove)

	summary.Append(actions)
	hero.Append(summary)

	details := adw.NewPreferencesGroup()
	details.SetTitle("Details")

	for _, field := range []struct {
		label string
		value string
	}{
		{"Update Status", w.updateStatus(info)},
		{"Version", orDash(info.Version)},
		{"Size", sizeOrDash(info.Size)},
		{"Update Source", sourceLabel(info.Source)},
		{"Signature", signatureLabel(info.Signature)},
		{"App ID", info.AppID},
		{"File", orDash(info.Filename)},
	} {
		row := adw.NewActionRow()
		row.SetTitle(field.label)
		row.SetSubtitle(field.value)
		details.Add(row)
	}

	content := gtk.NewBox(gtk.OrientationVertical, 0)
	content.Append(hero)

	clamp := adw.NewClamp()
	clamp.SetMaximumSize(560)
	clamp.SetTighteningThreshold(520)
	clamp.SetChild(details)
	clamp.SetMarginStart(24)
	clamp.SetMarginEnd(24)
	clamp.SetMarginBottom(24)
	content.Append(clamp)

	scroller := gtk.NewScrolledWindow()
	scroller.SetChild(content)
	scroller.SetVExpand(true)

	toolbar := adw.NewToolbarView()
	toolbar.AddTopBar(header)
	toolbar.SetContent(scroller)
	return toolbar
}

func appIcon(info appherder.AppInfo, size int) *gtk.Image {
	if info.Icon != "" && strings.HasPrefix(info.Icon, "/") {
		image := gtk.NewImageFromFile(info.Icon)
		image.SetPixelSize(size)
		image.SetSizeRequest(size, size)
		return image
	}
	iconName := info.Icon
	if iconName == "" {
		iconName = "application-x-executable-symbolic"
	}
	image := gtk.NewImageFromIconName(iconName)
	image.SetPixelSize(size)
	image.SetSizeRequest(size, size)
	return image
}

func (w *mainWindow) checkUpdates(userInitiated bool) {
	if w.checkingUpdates {
		return
	}
	w.checkingUpdates = true
	w.refreshVisibleApps()
	w.updateActionSensitivity()

	go func() {
		checks, err := w.app.CheckUpgrades(context.Background())
		w.idle(func() {
			w.checkingUpdates = false
			if err != nil {
				if userInitiated {
					w.showError("Could Not Check for Updates", err.Error())
				}
				w.updateActionSensitivity()
				w.refreshVisibleApps()
				return
			}
			w.updateChecks = checksByName(checks)
			w.checkedUpdates = true
			w.updateActionSensitivity()
			w.refreshVisibleApps()
		})
	}()
}

func (w *mainWindow) applyUpgrades() {
	w.run("Installing updates", func() (string, error) {
		checks := w.availableChecks()
		if len(checks) == 0 {
			var err error
			checks, err = w.app.CheckUpgrades(context.Background())
			if err != nil {
				return "", err
			}
		}
		applied := w.app.ApplyUpgrades(context.Background(), checks)
		upgraded, failed := summarizeApplied(applied)
		w.idle(func() {
			w.updateChecks = nil
			w.checkedUpdates = false
			w.loadApps()
			w.checkUpdates(false)
		})
		if upgraded == 0 && failed == 0 {
			return "Everything is up to date", nil
		}
		return fmt.Sprintf("%d app%s upgraded, %d upgrade%s failed", upgraded, plural(upgraded), failed, plural(failed)), nil
	})
}

func (w *mainWindow) updateApp(info appherder.AppInfo) {
	check, ok := w.updateCheck(info)
	if !ok || !check.Available {
		return
	}
	w.run("Updating "+info.Name, func() (string, error) {
		applied := w.app.ApplyUpgrades(context.Background(), []appherder.UpgradeCheck{check})
		if len(applied) == 0 {
			return info.Name + " is up to date", nil
		}
		if applied[0].Err != nil {
			return "", applied[0].Err
		}
		w.idle(func() {
			w.updateChecks = nil
			w.checkedUpdates = false
			w.loadApps()
			w.checkUpdates(false)
		})
		return fmt.Sprintf("Updated %s to %s", info.Name, applied[0].Version), nil
	})
}

func (w *mainWindow) launchApp(info appherder.AppInfo) {
	w.run("Launching "+info.Name, func() (string, error) {
		if err := w.app.Launch(appKey(info)); err != nil {
			return "", err
		}
		return "", nil
	})
}

func (w *mainWindow) promptInstallFile() {
	dialog := gtk.NewFileDialog()
	dialog.SetTitle("Install AppImage")

	filter := gtk.NewFileFilter()
	filter.SetName("AppImages")
	filter.AddSuffix("appimage")
	filter.AddSuffix("AppImage")

	allFiles := gtk.NewFileFilter()
	allFiles.SetName("All Files")
	allFiles.AddPattern("*")

	filters := gio.NewListStore(gtk.GTypeFileFilter)
	filters.Append(filter.Object)
	filters.Append(allFiles.Object)
	dialog.SetFilters(filters)
	dialog.SetDefaultFilter(filter)

	dialog.Open(context.Background(), &w.Window, func(result gio.AsyncResulter) {
		file, err := dialog.OpenFinish(result)
		if err != nil || file == nil {
			return
		}
		path := file.Path()
		if path == "" {
			w.showError("Cannot Install File", "Selected file has no local path.")
			return
		}
		w.install(path)
	})
}

func (w *mainWindow) promptInstallURL() {
	entry := gtk.NewEntry()
	entry.SetPlaceholderText("https://example.com/app.AppImage")
	entry.SetHExpand(true)

	dialog := adw.NewAlertDialog("Install from URL", "Enter an HTTP or HTTPS AppImage URL.")
	dialog.SetExtraChild(entry)
	dialog.AddResponse("cancel", "Cancel")
	dialog.AddResponse("install", "Install")
	dialog.SetCloseResponse("cancel")
	dialog.SetDefaultResponse("install")
	dialog.ConnectResponse(func(response string) {
		if response != "install" {
			return
		}
		target := strings.TrimSpace(entry.Text())
		if target == "" {
			w.showError("Cannot Install URL", "Enter a URL to install.")
			return
		}
		if !looksLikeURL(target) {
			w.showError("Cannot Install URL", "Enter an HTTP or HTTPS URL.")
			return
		}
		w.install(target)
	})
	dialog.Present(w)
}

func (w *mainWindow) installDroppedFiles(files []*gio.File) bool {
	if len(files) == 0 {
		return false
	}
	if len(files) > 1 {
		w.showError("Cannot Install Files", "Drop one AppImage at a time.")
		return false
	}
	path := files[0].Path()
	if path == "" {
		w.showError("Cannot Install File", "Drop a local AppImage file.")
		return false
	}
	if !isAppImagePath(path) {
		w.showError("Cannot Install File", "Drop a file ending in .AppImage.")
		return false
	}
	w.install(path)
	return true
}

func (w *mainWindow) install(target string) {
	w.run("Installing AppImage", func() (string, error) {
		var name string
		var err error
		if looksLikeURL(target) {
			name, err = w.app.InstallFromURL(context.Background(), target)
		} else {
			name, err = w.app.Install(target)
		}
		if err != nil {
			return "", err
		}
		w.idle(w.loadApps)
		return fmt.Sprintf("Installed %s", name), nil
	})
}

func (w *mainWindow) confirmUninstall(info appherder.AppInfo) {
	dialog := adw.NewAlertDialog("Remove "+info.Name+"?", "This removes the AppImage, launcher, icon, and appherder metadata.")
	dialog.AddResponse("cancel", "Cancel")
	dialog.AddResponse("remove", "Remove")
	dialog.SetCloseResponse("cancel")
	dialog.SetDefaultResponse("cancel")
	dialog.SetResponseAppearance("remove", adw.ResponseDestructive)
	dialog.ConnectResponse(func(response string) {
		if response == "remove" {
			w.uninstall(info)
		}
	})
	dialog.Present(w)
}

func (w *mainWindow) uninstall(info appherder.AppInfo) {
	w.run("Removing "+info.Name, func() (string, error) {
		if err := w.app.Uninstall(appKey(info), false); err != nil {
			return "", err
		}
		w.idle(w.loadApps)
		return "Removed " + info.Name, nil
	})
}

func (w *mainWindow) run(_ string, fn func() (string, error)) {
	w.setBusy(1)
	go func() {
		_, err := fn()
		w.idle(func() {
			w.setBusy(-1)
			if err != nil {
				w.showError("Operation Failed", err.Error())
				return
			}
		})
	}()
}

func (w *mainWindow) setBusy(delta int) {
	w.busy += delta
	if w.busy < 0 {
		w.busy = 0
	}
	sensitive := w.busy == 0
	w.installBtn.SetSensitive(sensitive)
	w.menuBtn.SetSensitive(sensitive)
	w.updateActionSensitivity()
}

func (w *mainWindow) updateActionSensitivity() {
	if w.checkAction != nil {
		w.checkAction.SetEnabled(w.busy == 0 && !w.checkingUpdates)
	}
	if w.updateAllAction != nil {
		w.updateAllAction.SetEnabled(w.busy == 0 && !w.checkingUpdates && len(w.availableChecks()) > 0)
	}
}

func (w *mainWindow) refreshVisibleApps() {
	if len(w.apps) == 0 {
		return
	}
	w.renderApps(w.apps)
	if w.currentKey == "" {
		return
	}
	for _, info := range w.apps {
		if appKey(info) == w.currentKey {
			w.showDetails(info, false)
			return
		}
	}
}

func (w *mainWindow) idle(fn func()) {
	glib.IdleAdd(fn)
}

func (w *mainWindow) showError(title, message string) {
	dialog := adw.NewAlertDialog(title, message)
	dialog.AddResponse("close", "Close")
	dialog.SetCloseResponse("close")
	dialog.SetDefaultResponse("close")
	dialog.Present(w)
}

func looksLikeURL(s string) bool {
	parsed, err := url.Parse(s)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func isAppImagePath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".appimage")
}

func (w *mainWindow) updateAvailable(info appherder.AppInfo) bool {
	check, ok := w.updateCheck(info)
	return ok && check.Available
}

func (w *mainWindow) updateStatus(info appherder.AppInfo) string {
	if w.checkingUpdates {
		return "Checking for updates"
	}
	check, ok := w.updateCheck(info)
	if !ok {
		if w.checkedUpdates {
			return "Not managed by AppHerder"
		}
		return "Not checked yet"
	}
	if check.Err != nil {
		return "Could not check"
	}
	if check.NoSource {
		return "Unavailable"
	}
	if check.Available {
		return "Update available: " + orDash(check.Release.Version)
	}
	return "Up to date"
}

func (w *mainWindow) updateCheck(info appherder.AppInfo) (appherder.UpgradeCheck, bool) {
	if w.updateChecks == nil {
		return appherder.UpgradeCheck{}, false
	}
	check, ok := w.updateChecks[upgradeKey(info)]
	return check, ok
}

func (w *mainWindow) availableChecks() []appherder.UpgradeCheck {
	if w.updateChecks == nil {
		return nil
	}
	checks := make([]appherder.UpgradeCheck, 0, len(w.updateChecks))
	for _, check := range w.updateChecks {
		if check.Err == nil && !check.NoSource && check.Available {
			checks = append(checks, check)
		}
	}
	return checks
}

func checksByName(checks []appherder.UpgradeCheck) map[string]appherder.UpgradeCheck {
	byName := make(map[string]appherder.UpgradeCheck, len(checks))
	for _, check := range checks {
		byName[check.Name] = check
	}
	return byName
}

func summarizeApplied(applied []appherder.UpgradeApplied) (upgraded, failed int) {
	for _, app := range applied {
		if app.Err != nil {
			failed++
		} else {
			upgraded++
		}
	}
	return upgraded, failed
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for scaled := bytes / unit; scaled >= unit; scaled /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func appKey(info appherder.AppInfo) string {
	if info.AppID != "" {
		return info.AppID
	}
	if info.Filename == "" {
		return appherder.NormalizeAppName(info.Name)
	}
	return strings.TrimSuffix(filepath.Base(info.Filename), filepath.Ext(info.Filename))
}

func upgradeKey(info appherder.AppInfo) string {
	if info.Filename != "" {
		return strings.TrimSuffix(filepath.Base(info.Filename), filepath.Ext(info.Filename))
	}
	return appKey(info)
}

func sizeOrDash(bytes int64) string {
	if bytes <= 0 {
		return "-"
	}
	return humanSize(bytes)
}

func sourceLabel(source string) string {
	switch source {
	case "", "none":
		return "No update source"
	case "github":
		return "GitHub"
	case "gitlab":
		return "GitLab"
	case "zsync":
		return "zsync"
	case "static":
		return "Static URL"
	default:
		return source
	}
}

func signatureLabel(signature string) string {
	switch signature {
	case "pinned":
		return "Pinned signature"
	case "signed":
		return "Signed"
	case "", "none":
		return "Unsigned"
	default:
		return signature
	}
}
