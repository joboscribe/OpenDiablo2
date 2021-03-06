package d2player

import (
	"image/color"
	"log"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2data/d2datadict"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2input"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2term"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2ui"
)

type Panel interface {
	IsOpen() bool
	Toggle()
	Open()
	Close()
}

// ID of missile to create when user right clicks.
var missileID = 59

type GameControls struct {
	hero          *d2mapentity.Player
	mapEngine     *d2mapengine.MapEngine
	mapRenderer   *d2maprenderer.MapRenderer
	inventory     *Inventory
	heroStats     *HeroStats
	escapeMenu    *EscapeMenu
	inputListener InputCallbackListener
	FreeCam       bool

	// UI
	globeSprite       *d2ui.Sprite
	mainPanel         *d2ui.Sprite
	menuButton        *d2ui.Sprite
	skillIcon         *d2ui.Sprite
	zoneChangeText    *d2ui.Label
	runButton         d2ui.Button
	isZoneTextShown   bool
	actionableRegions []ActionableRegion
}

type ActionableType int
type ActionableRegion struct {
	ActionableTypeId ActionableType
	Rect             d2common.Rectangle
}

const (
	// Since they require special handling, not considering (1) globes, (2) content of the mini panel, (3) belt
	leftSkill  = ActionableType(iota)
	leftSelec  = ActionableType(iota)
	xp         = ActionableType(iota)
	walkRun    = ActionableType(iota)
	stamina    = ActionableType(iota)
	miniPanel  = ActionableType(iota)
	rightSelec = ActionableType(iota)
	rightSkill = ActionableType(iota)
)

func NewGameControls(hero *d2mapentity.Player, mapEngine *d2mapengine.MapEngine, mapRenderer *d2maprenderer.MapRenderer, inputListener InputCallbackListener) *GameControls {
	d2term.BindAction("setmissile", "set missile id to summon on right click", func(id int) {
		missileID = id
	})

	label := d2ui.CreateLabel(d2resource.Font30, d2resource.PaletteUnits)
	label.Color = color.RGBA{R: 255, G: 88, B: 82, A: 255}
	label.Alignment = d2ui.LabelAlignCenter

	gc := &GameControls{
		hero:           hero,
		mapEngine:      mapEngine,
		inputListener:  inputListener,
		mapRenderer:    mapRenderer,
		inventory:      NewInventory(),
		heroStats:      NewHeroStats(),
		escapeMenu:     NewEscapeMenu(),
		zoneChangeText: &label,
		actionableRegions: []ActionableRegion{
			{leftSkill, d2common.Rectangle{Left: 115, Top: 550, Width: 50, Height: 50}},
			{leftSelec, d2common.Rectangle{Left: 206, Top: 563, Width: 30, Height: 30}},
			{xp, d2common.Rectangle{Left: 253, Top: 560, Width: 125, Height: 5}},
			{walkRun, d2common.Rectangle{Left: 255, Top: 573, Width: 17, Height: 20}},
			{stamina, d2common.Rectangle{Left: 273, Top: 573, Width: 105, Height: 20}},
			{miniPanel, d2common.Rectangle{Left: 393, Top: 563, Width: 12, Height: 23}},
			{rightSelec, d2common.Rectangle{Left: 562, Top: 563, Width: 30, Height: 30}},
			{rightSkill, d2common.Rectangle{Left: 634, Top: 550, Width: 50, Height: 50}},
		},
	}

	d2term.BindAction("freecam", "toggle free camera movement", func() {
		gc.FreeCam = !gc.FreeCam
	})

	return gc
}

func (g *GameControls) OnKeyRepeat(event d2input.KeyEvent) bool {
	if g.FreeCam {
		var moveSpeed float64 = 8
		if event.KeyMod == d2input.KeyModShift {
			moveSpeed *= 2
		}

		if event.Key == d2input.KeyDown {
			g.mapRenderer.MoveCameraBy(0, moveSpeed)
			return true
		}

		if event.Key == d2input.KeyUp {
			g.mapRenderer.MoveCameraBy(0, -moveSpeed)
			return true
		}

		if event.Key == d2input.KeyRight {
			g.mapRenderer.MoveCameraBy(moveSpeed, 0)
			return true
		}

		if event.Key == d2input.KeyLeft {
			g.mapRenderer.MoveCameraBy(-moveSpeed, 0)
			return true
		}
	}

	return false
}

func (g *GameControls) OnKeyDown(event d2input.KeyEvent) bool {
	switch event.Key {
	case d2input.KeyEscape:
		if g.inventory.IsOpen() || g.heroStats.IsOpen() {
			g.inventory.Close()
			g.heroStats.Close()
			g.updateLayout()
			break
		}
		g.escapeMenu.Toggle()
	case d2input.KeyUp:
		g.escapeMenu.OnUpKey()
	case d2input.KeyDown:
		g.escapeMenu.OnDownKey()
	case d2input.KeyEnter:
		g.escapeMenu.OnEnterKey()
	case d2input.KeyI:
		g.inventory.Toggle()
		g.updateLayout()
	case d2input.KeyC:
		g.heroStats.Toggle()
		g.updateLayout()
	case d2input.KeyR:
		g.onToggleRunButton()
	default:
		return false
	}
	return false
}

var lastLeftBtnActionTime float64 = 0
var lastRightBtnActionTime float64 = 0
var mouseBtnActionsTreshhold = 0.25

func (g *GameControls) OnMouseButtonRepeat(event d2input.MouseEvent) bool {
	px, py := g.mapRenderer.ScreenToWorld(event.X, event.Y)
	px = float64(int(px*10)) / 10.0
	py = float64(int(py*10)) / 10.0

	now := d2common.Now()
	if event.Button == d2input.MouseButtonLeft && now-lastLeftBtnActionTime >= mouseBtnActionsTreshhold {
		lastLeftBtnActionTime = now
		g.inputListener.OnPlayerMove(px, py)
		return true
	}

	if event.Button == d2input.MouseButtonRight && now-lastRightBtnActionTime >= mouseBtnActionsTreshhold {
		lastRightBtnActionTime = now
		g.ShootMissile(px, py)
		return true
	}

	return true
}

func (g *GameControls) OnMouseMove(event d2input.MouseMoveEvent) bool {
	if g.escapeMenu.IsOpen() {
		g.escapeMenu.OnMouseMove(event)
		return false
	}

	mx, my := event.X, event.Y
	for i := range g.actionableRegions {
		// Mouse over a game control element
		if g.actionableRegions[i].Rect.IsInRect(mx, my) {
			g.onHoverActionable(g.actionableRegions[i].ActionableTypeId)
		}
	}

	return false
}

func (g *GameControls) OnMouseButtonDown(event d2input.MouseEvent) bool {
	if g.escapeMenu.IsOpen() {
		return g.escapeMenu.OnMouseButtonDown(event)
	}

	mx, my := event.X, event.Y
	for i := range g.actionableRegions {
		// If click is on a game control element
		if g.actionableRegions[i].Rect.IsInRect(mx, my) {
			g.onClickActionable(g.actionableRegions[i].ActionableTypeId)
			return false
		}
	}

	px, py := g.mapRenderer.ScreenToWorld(mx, my)
	px = float64(int(px*10)) / 10.0
	py = float64(int(py*10)) / 10.0

	if event.Button == d2input.MouseButtonLeft {
		lastLeftBtnActionTime = d2common.Now()
		g.inputListener.OnPlayerMove(px, py)
		return true
	}

	if event.Button == d2input.MouseButtonRight {
		lastRightBtnActionTime = d2common.Now()
		return g.ShootMissile(px, py)
	}

	return false
}

func (g *GameControls) ShootMissile(px float64, py float64) bool {
	missile, err := d2mapentity.CreateMissile(
		int(g.hero.LocationX),
		int(g.hero.LocationY),
		d2datadict.Missiles[missileID],
	)
	if err != nil {
		return false
	}

	rads := d2common.GetRadiansBetween(
		g.hero.LocationX,
		g.hero.LocationY,
		px*5,
		py*5,
	)

	missile.SetRadians(rads, func() {
		g.mapEngine.RemoveEntity(missile)
	})

	g.mapEngine.AddEntity(missile)
	return true
}

func (g *GameControls) Load() {
	animation, _ := d2asset.LoadAnimation(d2resource.GameGlobeOverlap, d2resource.PaletteSky)
	g.globeSprite, _ = d2ui.LoadSprite(animation)

	animation, _ = d2asset.LoadAnimation(d2resource.GamePanels, d2resource.PaletteSky)
	g.mainPanel, _ = d2ui.LoadSprite(animation)

	animation, _ = d2asset.LoadAnimation(d2resource.MenuButton, d2resource.PaletteSky)
	g.menuButton, _ = d2ui.LoadSprite(animation)

	animation, _ = d2asset.LoadAnimation(d2resource.GenericSkills, d2resource.PaletteSky)
	g.skillIcon, _ = d2ui.LoadSprite(animation)

	g.loadUIButtons()

	g.inventory.Load()
	g.heroStats.Load()
	g.escapeMenu.OnLoad()
}

func (g *GameControls) loadUIButtons() {
	// Run button
	g.runButton = d2ui.CreateButton(d2ui.ButtonTypeRun, "")
	g.runButton.SetPosition(255, 570)
	g.runButton.OnActivated(func() { g.onToggleRunButton() })
	if g.hero.IsRunToggled() {
		g.runButton.Toggle()
	}
	d2ui.AddWidget(&g.runButton)
}

func (g *GameControls) onToggleRunButton() {
	g.runButton.Toggle()
	g.hero.ToggleRunWalk()
	// TODO: change the running menu icon
	g.hero.SetIsRunning(g.hero.IsRunToggled())
}

// ScreenAdvanceHandler
func (g *GameControls) Advance(elapsed float64) error {
	g.escapeMenu.Advance(elapsed)
	return nil
}

func (g *GameControls) updateLayout() {
	isRightPanelOpen := false
	isLeftPanelOpen := false

	// todo : add same logic when adding quest log and skill tree
	isRightPanelOpen = g.inventory.isOpen || isRightPanelOpen
	isLeftPanelOpen = g.heroStats.isOpen || isLeftPanelOpen

	if isRightPanelOpen == isLeftPanelOpen {
		g.mapRenderer.ViewportDefault()
	} else if isRightPanelOpen == true {
		g.mapRenderer.ViewportToLeft()
	} else {
		g.mapRenderer.ViewportToRight()
	}
}

// TODO: consider caching the panels to single image that is reused.
func (g *GameControls) Render(target d2render.Surface) {
	g.inventory.Render(target)
	g.heroStats.Render(target)
	g.escapeMenu.Render(target)

	width, height := target.GetSize()
	offset := 0

	// Left globe holder
	g.mainPanel.SetCurrentFrame(0)
	w, _ := g.mainPanel.GetCurrentFrameSize()
	g.mainPanel.SetPosition(offset, height)
	g.mainPanel.Render(target)

	// Left globe
	g.globeSprite.SetCurrentFrame(0)
	g.globeSprite.SetPosition(offset+28, height-5)
	g.globeSprite.Render(target)
	offset += w

	// Left skill
	g.skillIcon.SetCurrentFrame(2)
	w, _ = g.skillIcon.GetCurrentFrameSize()
	g.skillIcon.SetPosition(offset, height)
	g.skillIcon.Render(target)
	offset += w

	// Left skill selector
	g.mainPanel.SetCurrentFrame(1)
	w, _ = g.mainPanel.GetCurrentFrameSize()
	g.mainPanel.SetPosition(offset, height)
	g.mainPanel.Render(target)
	offset += w

	// Stamina
	g.mainPanel.SetCurrentFrame(2)
	w, _ = g.mainPanel.GetCurrentFrameSize()
	g.mainPanel.SetPosition(offset, height)
	g.mainPanel.Render(target)
	offset += w

	// Center menu button
	g.menuButton.SetCurrentFrame(0)
	w, _ = g.mainPanel.GetCurrentFrameSize()
	g.menuButton.SetPosition((width/2)-8, height-16)
	g.menuButton.Render(target)

	// Potions
	g.mainPanel.SetCurrentFrame(3)
	w, _ = g.mainPanel.GetCurrentFrameSize()
	g.mainPanel.SetPosition(offset, height)
	g.mainPanel.Render(target)
	offset += w

	// Right skill selector
	g.mainPanel.SetCurrentFrame(4)
	w, _ = g.mainPanel.GetCurrentFrameSize()
	g.mainPanel.SetPosition(offset, height)
	g.mainPanel.Render(target)
	offset += w

	// Right skill
	g.skillIcon.SetCurrentFrame(10)
	w, _ = g.skillIcon.GetCurrentFrameSize()
	g.skillIcon.SetPosition(offset, height)
	g.skillIcon.Render(target)
	offset += w

	// Right globe holder
	g.mainPanel.SetCurrentFrame(5)
	w, _ = g.mainPanel.GetCurrentFrameSize()
	g.mainPanel.SetPosition(offset, height)
	g.mainPanel.Render(target)

	// Right globe
	g.globeSprite.SetCurrentFrame(1)
	g.globeSprite.SetPosition(offset+8, height-8)
	g.globeSprite.Render(target)

	if g.isZoneTextShown {
		g.zoneChangeText.SetPosition(width/2, height/4)
		g.zoneChangeText.Render(target)
	}
}

func (g *GameControls) SetZoneChangeText(text string) {
	g.zoneChangeText.SetText(text)
}

func (g *GameControls) ShowZoneChangeText() {
	g.isZoneTextShown = true
}

func (g *GameControls) HideZoneChangeTextAfter(delay float64) {
	time.AfterFunc(time.Duration(delay)*time.Second, func() {
		g.isZoneTextShown = false
	})
}

func (g *GameControls) InEscapeMenu() bool {
	return g != nil && g.escapeMenu != nil && g.escapeMenu.IsOpen()
}

// Handles what to do when an actionable is hovered
func (g *GameControls) onHoverActionable(item ActionableType) {
	switch item {
	case leftSkill:
		return
	case leftSelec:
		return
	case xp:
		return
	case walkRun:
		return
	case stamina:
		return
	case miniPanel:
		return
	case rightSelec:
		return
	case rightSkill:
		return
	default:
		log.Printf("Unrecognized ActionableType(%d) being hovered\n", item)
	}
}

// Handles what to do when an actionable is clicked
func (g *GameControls) onClickActionable(item ActionableType) {
	switch item {
	case leftSkill:
		log.Println("Left Skill Action Pressed")
	case leftSelec:
		log.Println("Left Skill Selector Action Pressed")
	case xp:
		log.Println("XP Action Pressed")
	case walkRun:
		log.Println("Walk/Run Action Pressed")
	case stamina:
		log.Println("Stamina Action Pressed")
	case miniPanel:
		log.Println("Mini Panel Action Pressed")
	case rightSelec:
		log.Println("Right Skill Selector Action Pressed")
	case rightSkill:
		log.Println("Right Skill Action Pressed")
	default:
		log.Printf("Unrecognized ActionableType(%d) being clicked\n", item)
	}
}
