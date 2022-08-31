package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func init() {
	sys.gameState.randseed = sys.randseed
}

type ReplayState struct {
	state      *bytes.Buffer
	ib         [MaxSimul*2 + MaxAttachedChar]InputBits
	buf        [MaxSimul*2 + MaxAttachedChar]NetBuffer
	delay      int32
	locIn      int
	remIn      int
	time       int32
	stoppedcnt int32
}

func (rs *ReplayState) RecordInput(cb *CommandBuffer, i int, facing int32) {
	fmt.Println("RecordInput.")

	if i >= 0 && i < len(rs.buf) {
		rs.buf[sys.inputRemap[i]].input(cb, facing)
	}
}

func (rs *ReplayState) PlayInput(cb *CommandBuffer, i int, facing int32) {
	fmt.Println("PlayInput.")

	if i >= 0 && i < len(rs.ib) {
		rs.ib[sys.inputRemap[i]].GetInput(cb, facing)
	}
}

func (rs *ReplayState) PlayAnyButton() bool {
	fmt.Println("PlayAnyButton.")

	for _, b := range rs.ib {
		if b&IB_anybutton != 0 {
			return true
		}
	}
	return false
}
func (rs *ReplayState) RecordUpdate() bool {
	fmt.Println("RecordUpdate.")

	if !sys.gameEnd {
		fmt.Println("I'm here in RecordUpdateLoop.")
		rs.buf[rs.locIn].localUpdate(0)
		if rs.state != nil {
			for _, nb := range rs.buf {
				fmt.Println("I wrote to some state.")
				binary.Write(rs.state, binary.LittleEndian, &nb.buf[rs.time&31])
			}
		}
		rs.time++
	}
	return !sys.gameEnd
}

func (rs *ReplayState) RecordAnyButton() bool {
	fmt.Println("RecordAnyButton.")

	for _, nb := range rs.buf {
		if nb.buf[nb.curT&31]&IB_anybutton != 0 {
			return true
		}
	}
	return false
}

func (rs *ReplayState) PlayUpdate() bool {
	fmt.Println("PlayUpdate.")
	if sys.oldNextAddTime > 0 &&
		binary.Read(rs.state, binary.LittleEndian, rs.ib[:]) != nil {
		sys.playReplayState = false
		rs = nil
	}
	return !sys.gameEnd
}

func NewReplayState() ReplayState {
	return ReplayState{
		state: bytes.NewBuffer([]byte{}),
	}
}

type CharState struct {
	palFX          PalFX
	childrenState  []CharState
	enemynearState [2][]CharState

	animState       AnimationState
	cmd             []CommandList
	ss              StateState
	hitdef          HitDef
	redLife         int32
	juggle          int32
	life            int32
	name            string
	key             int
	id              int32
	helperId        int32
	helperIndex     int32
	parentIndex     int32
	playerNo        int
	teamside        int
	player          bool
	animPN          int
	animNo          int32
	lifeMax         int32
	powerMax        int32
	dizzyPoints     int32
	dizzyPointsMax  int32
	guardPoints     int32
	guardPointsMax  int32
	fallTime        int32
	localcoord      float32
	localscl        float32
	clsnScale       [2]float32
	hoIdx           int
	mctime          int32
	targets         []int32
	targetsOfHitdef []int32
	pos             [3]float32
	drawPos         [3]float32
	oldPos          [3]float32
	vel             [3]float32
	facing          float32
	CharSystemVar
	p1facing              float32
	cpucmd                int32
	attackDist            float32
	stchtmp               bool
	inguarddist           bool
	pushed                bool
	hitdefContact         bool
	atktmp                int8
	hittmp                int8
	acttmp                int8
	minus                 int8
	platformPosY          float32
	groundAngle           float32
	ownpal                bool
	winquote              int32
	memberNo              int
	selectNo              int
	comboExtraFrameWindow int32
	inheritJuggle         int32
	immortal              bool
	kovelocity            bool
	preserve              int32
	inputFlag             InputBits
	pauseBool             bool

	keyctrl         [4]bool
	power           int32
	size            CharSize
	ghv             GetHitVar
	hitby           [2]HitBy
	ho              [8]HitOverride
	mctype          MoveContact
	ivar            [NumVar + NumSysVar]int32
	fvar            [NumFvar + NumSysFvar]float32
	offset          [2]float32
	mapArray        map[string]float32
	mapDefault      map[string]float32
	remapSpr        RemapPreset
	clipboardText   []string
	dialogue        []string
	defaultHitScale [3]*HitScale
	nextHitScale    map[int32][3]*HitScale
	activeHitScale  map[int32][3]*HitScale
}

func (cs *CharState) findChar() *Char {
	for i := 0; i < len(sys.chars); i++ {
		for j := 0; j < len(sys.chars[i]); j++ {
			if cs.id == sys.chars[i][j].id {
				return sys.chars[i][j]
			}
		}
	}
	return nil
}

type ExplodState struct {
	animState      AnimationState
	id             int32
	bindtime       int32
	scale          [2]float32
	time           int32
	removeongethit bool
	removetime     int32
	velocity       [2]float32
	accel          [2]float32
	sprpriority    int32
	postype        PosType
	space          Space
	offset         [2]float32
	relativef      int32
	pos            [2]float32
	facing         float32
	vfacing        float32
	shadow         [3]int32
	supermovetime  int32
	pausemovetime  int32
	ontop          bool
	under          bool
	alpha          [2]int32
	ownpal         bool
	playerId       int32
	bindId         int32
	ignorehitpause bool
	rot            Rotation
	projection     Projection
	fLength        float32
	oldPos         [2]float32
	newPos         [2]float32
	palfxdef       PalFXDef
	window         [4]float32
	localscl       float32
	palFX          PalFX
}

type AnimationState struct {
	ptr                *Animation
	frames             []AnimFrame
	tile               Tiling
	loopstart          int32
	interpolate_offset []int32
	interpolate_scale  []int32
	interpolate_angle  []int32
	interpolate_blend  []int32
	// Current frame
	current                    int32
	drawidx                    int32
	time                       int32
	sumtime                    int32
	totaltime                  int32
	looptime                   int32
	nazotime                   int32
	mask                       int16
	srcAlpha                   int16
	dstAlpha                   int16
	newframe                   bool
	loopend                    bool
	interpolate_offset_x       float32
	interpolate_offset_y       float32
	scale_x                    float32
	scale_y                    float32
	angle                      float32
	interpolate_blend_srcalpha float32
	interpolate_blend_dstalpha float32
	remap                      RemapPreset
	start_scale                [2]float32
}
type ProjectileState struct {
	ptr             *Projectile
	hitdef          HitDef
	id              int32
	anim            int32
	anim_fflg       bool
	hitanim         int32
	hitanim_fflg    bool
	remanim         int32
	remanim_fflg    bool
	cancelanim      int32
	cancelanim_fflg bool
	scale           [2]float32
	angle           float32
	clsnScale       [2]float32
	remove          bool
	removetime      int32
	velocity        [2]float32
	remvelocity     [2]float32
	accel           [2]float32
	velmul          [2]float32
	hits            int32
	misstime        int32
	priority        int32
	priorityPoints  int32
	sprpriority     int32
	edgebound       int32
	stagebound      int32
	heightbound     [2]int32
	pos             [2]float32
	facing          float32
	removefacing    float32
	shadow          [3]int32
	supermovetime   int32
	pausemovetime   int32
	timemiss        int32
	hitpause        int32
	oldPos          [2]float32
	newPos          [2]float32
	aimg            AfterImage
	localscl        float32
	parentAttackmul float32
	platform        bool
	platformWidth   [2]float32
	platformHeight  [2]float32
	platformAngle   float32
	platformFence   bool
}
type StageState struct {
	p           [2]stagePlayer
	sdw         stageShadow
	leftbound   float32
	rightbound  float32
	screenleft  int32
	screenright int32
	stageCamera stageCamera
	zoffsetlink int32
	scale       [2]float32
	reflection  int32
	stageTime   int32
}

type GameState struct {
	randseed          int32
	time              int32
	gameTime          int32
	projectileState   [MaxSimul*2 + MaxAttachedChar][]ProjectileState
	charState         [MaxSimul*2 + MaxAttachedChar][]CharState
	explodsState      [MaxSimul*2 + MaxAttachedChar][]ExplodState
	explDrawlist      [MaxSimul*2 + MaxAttachedChar][]int
	topexplDrawlist   [MaxSimul*2 + MaxAttachedChar][]int
	underexplDrawlist [MaxSimul*2 + MaxAttachedChar][]int
	aiInput           [MaxSimul*2 + MaxAttachedChar]AiInput
	inputRemap        [MaxSimul*2 + MaxAttachedChar]int
	autoguard         [MaxSimul*2 + MaxAttachedChar]bool

	charMap map[int32]CharState
	projMap map[int32]ProjectileState

	com                [MaxSimul*2 + MaxAttachedChar]float32
	cam                Camera
	allPalFX           PalFX
	bgPalFX            PalFX
	pause              int32
	pausetime          int32
	pausebg            bool
	pauseendcmdbuftime int32
	pauseplayer        int
	super              int32
	supertime          int32
	superpausebg       bool
	superendcmdbuftime int32
	superplayer        int
	superdarken        bool
	superanim          AnimationState
	superanimRef       *Animation
	superpmap          PalFX
	superpos           [2]float32
	superfacing        float32
	superp2defmul      float32

	envShake            EnvShake
	specialFlag         GlobalSpecialFlag
	envcol              [3]int32
	envcol_time         int32
	bcStack, bcVarStack BytecodeStack
	bcVar               []BytecodeValue
	stageState          StageState
	workBe              []BytecodeExp

	scrrect                 [4]int32
	gameWidth, gameHeight   int32
	widthScale, heightScale float32
	gameEnd, frameSkip      bool
	brightness              int32
	roundTime               int32
	lifeMul                 float32
	team1VS2Life            float32
	turnsRecoveryRate       float32
	match                   int32
	round                   int32
	intro                   int32
	lastHitter              [2]int
	winTeam                 int
	winType                 [2]WinType
	winTrigger              [2]WinType
	matchWins, wins         [2]int32
	roundsExisted           [2]int32
	draws                   int32
	tmode                   [2]TeamMode
	numSimul, numTurns      [2]int32
	esc                     bool
	workingCharState        CharState
	workingStateState       StateBytecode
	afterImageMax           int32
	comboExtraFrameWindow   int32
	envcol_under            bool
	helperMax               int32
	nextCharId              int32
	powerShare              [2]bool
	tickCount               int
	oldTickCount            int
	tickCountF              float32
	lastTick                float32
	nextAddTime             float32
	oldNextAddTime          float32
	screenleft              float32
	screenright             float32
	xmin, xmax              float32
	winskipped              bool
	paused, step            bool
	roundResetFlg           bool
	reloadFlg               bool
	reloadStageFlg          bool
	reloadLifebarFlg        bool
	reloadCharSlot          [MaxSimul*2 + MaxAttachedChar]bool
	turbo                   float32
	drawScale               float32
	zoomlag                 float32
	zoomScale               float32
	zoomPosXLag             float32
	zoomPosYLag             float32
	enableZoomstate         bool
	zoomCameraBound         bool
	zoomPos                 [2]float32
	finish                  FinishType
	waitdown                int32
	slowtime                int32
	shuttertime             int32
	fadeintime              int32
	fadeouttime             int32
	changeStateNest         int32
	drawc1                  ClsnRect
	drawc2                  ClsnRect
	drawc2sp                ClsnRect
	drawc2mtk               ClsnRect
	drawwh                  ClsnRect
	accel                   float32
	clsnDraw                bool
	statusDraw              bool
	explodMax               int
	workpal                 []uint32
	playerProjectileMax     int
	nomusic                 bool
	lifeShare               [2]bool
}

func NewGameState() GameState {
	return GameState{
		charMap: make(map[int32]CharState),
		projMap: make(map[int32]ProjectileState),
	}
}

func (gs *GameState) Equal(other GameState) (equality bool, unequal string) {

	if gs.randseed != other.randseed {
		return false, fmt.Sprintf("randseed: %d: %d", gs.randseed, other.randseed)
	}

	if gs.time != other.time {
		return false, fmt.Sprintf("time: %d: %d", gs.time, other.time)
	}

	if gs.gameTime != other.gameTime {
		return false, fmt.Sprintf("gameTime: %d: %d", gs.gameTime, other.gameTime)
	}
	return true, ""

}

func (gs *GameState) LoadState() {
	sys.randseed = gs.randseed
	sys.time = gs.time
	sys.gameTime = gs.gameTime
	gs.loadCharData()
	gs.loadExplodData()
	sys.cam = gs.cam
	gs.loadPauseData()
	gs.loadSuperData()
	gs.loadPalFX()
	gs.loadProjectileData()
	sys.com = gs.com
	sys.envShake = gs.envShake
	sys.envcol_time = gs.envcol_time
	sys.specialFlag = gs.specialFlag
	sys.envcol = gs.envcol
	sys.bcStack = gs.bcStack
	sys.bcVarStack = gs.bcVarStack
	sys.bcVar = gs.bcVar
	sys.stage.loadStageState(gs.stageState)
	sys.aiInput = gs.aiInput
	sys.inputRemap = gs.inputRemap
	sys.autoguard = gs.autoguard
	sys.workBe = gs.workBe

	sys.finish = gs.finish
	sys.winTeam = gs.winTeam
	sys.winType = gs.winType
	sys.winTrigger = gs.winTrigger
	sys.lastHitter = gs.lastHitter
	sys.waitdown = gs.waitdown
	sys.slowtime = gs.slowtime
	sys.shuttertime = gs.shuttertime
	sys.fadeintime = gs.fadeintime
	sys.fadeouttime = gs.fadeouttime
	sys.winskipped = gs.winskipped
	sys.intro = gs.intro
	sys.time = gs.time
	sys.nextCharId = gs.nextCharId

	sys.scrrect = gs.scrrect
	sys.gameWidth = gs.gameWidth
	sys.gameHeight = gs.gameHeight
	sys.widthScale = gs.widthScale
	sys.heightScale = gs.heightScale
	sys.gameEnd = gs.gameEnd
	sys.frameSkip = gs.frameSkip
	sys.brightness = gs.brightness
	sys.roundTime = gs.roundTime
	sys.lifeMul = gs.lifeMul
	sys.team1VS2Life = gs.team1VS2Life
	sys.turnsRecoveryRate = gs.turnsRecoveryRate

	sys.changeStateNest = gs.changeStateNest
	copy(sys.drawc1, gs.drawc1)
	copy(sys.drawc2, gs.drawc2)
	copy(sys.drawc2sp, gs.drawc2sp)
	copy(sys.drawc2mtk, gs.drawc2mtk)
	copy(sys.drawwh, gs.drawwh)
	sys.accel = gs.accel
	sys.clsnDraw = gs.clsnDraw
	sys.statusDraw = gs.statusDraw
	sys.explodMax = gs.explodMax
	copy(sys.workpal, gs.workpal)
	sys.playerProjectileMax = gs.playerProjectileMax
	sys.nomusic = gs.nomusic
	sys.lifeShare = gs.lifeShare

	sys.turbo = gs.turbo
	sys.drawScale = gs.drawScale
	sys.zoomlag = gs.zoomlag
	sys.zoomScale = gs.zoomScale
	sys.zoomPosXLag = gs.zoomPosXLag
	sys.zoomPosYLag = gs.zoomPosYLag
	sys.enableZoomstate = gs.enableZoomstate
	sys.zoomCameraBound = gs.zoomCameraBound
	sys.zoomPos = gs.zoomPos

	sys.reloadCharSlot = gs.reloadCharSlot
	sys.turbo = gs.turbo
	sys.drawScale = gs.drawScale
	sys.zoomlag = gs.zoomlag
	sys.zoomScale = gs.zoomScale
	sys.zoomPosXLag = gs.zoomPosXLag

	sys.matchWins = gs.matchWins
	sys.wins = gs.wins
	sys.roundsExisted = gs.roundsExisted
	sys.draws = gs.draws
	sys.tmode = gs.tmode
	sys.numSimul = gs.numSimul
	sys.numTurns = gs.numTurns
	sys.esc = gs.esc
	sys.afterImageMax = gs.afterImageMax
	sys.comboExtraFrameWindow = gs.comboExtraFrameWindow
	sys.envcol_under = gs.envcol_under
	sys.helperMax = gs.helperMax
	sys.nextCharId = gs.nextCharId
	sys.powerShare = gs.powerShare
	sys.tickCount = gs.tickCount
	sys.oldTickCount = gs.oldTickCount
	sys.tickCountF = gs.tickCountF
	sys.lastTick = gs.lastTick
	sys.nextAddTime = gs.nextAddTime
	sys.oldNextAddTime = gs.oldNextAddTime
	sys.screenleft = gs.screenleft
	sys.screenright = gs.screenright
	sys.xmin = gs.xmin
	sys.xmax = gs.xmax
	sys.winskipped = gs.winskipped
	sys.paused = gs.paused
	sys.step = gs.step
	sys.roundResetFlg = gs.roundResetFlg
	sys.reloadFlg = gs.reloadFlg
	sys.reloadStageFlg = gs.reloadStageFlg
	sys.reloadLifebarFlg = gs.reloadLifebarFlg

	sys.match = gs.match
	sys.round = gs.round

	if sys.workingState != nil {
		*sys.workingState = gs.workingStateState
	}
	// else {
	// 	sys.workingState = &gs.workingStateState
	// }
}

func (gs *GameState) SaveState() {
	gs.randseed = sys.randseed
	gs.time = sys.time
	gs.gameTime = sys.gameTime
	gs.saveCharData()
	gs.saveExplodData()
	gs.cam = sys.cam
	gs.savePauseData()
	gs.saveSuperData()
	gs.savePalFX()
	gs.saveProjectileData()
	gs.com = sys.com
	gs.envShake = sys.envShake
	gs.envcol_time = sys.envcol_time
	gs.specialFlag = sys.specialFlag
	gs.envcol = sys.envcol
	gs.bcStack = sys.bcStack
	gs.bcVarStack = sys.bcVarStack
	gs.bcVar = sys.bcVar
	gs.stageState = sys.stage.getStageState()
	gs.aiInput = sys.aiInput
	gs.inputRemap = sys.inputRemap
	gs.autoguard = sys.autoguard
	gs.workBe = make([]BytecodeExp, len(sys.workBe))
	copy(gs.workBe, sys.workBe)

	gs.finish = sys.finish
	gs.winTeam = sys.winTeam
	gs.winType = sys.winType
	gs.winTrigger = sys.winTrigger
	gs.lastHitter = sys.lastHitter
	gs.waitdown = sys.waitdown
	gs.slowtime = sys.slowtime
	gs.shuttertime = sys.shuttertime
	gs.fadeintime = sys.fadeintime
	gs.fadeouttime = sys.fadeouttime
	gs.winskipped = sys.winskipped
	gs.intro = sys.intro
	gs.time = sys.time
	gs.nextCharId = sys.nextCharId

	gs.scrrect = sys.scrrect
	gs.gameWidth = sys.gameWidth
	gs.gameHeight = sys.gameHeight
	gs.widthScale = sys.widthScale
	gs.heightScale = sys.heightScale
	gs.gameEnd = sys.gameEnd
	gs.frameSkip = sys.frameSkip
	gs.brightness = sys.brightness
	gs.roundTime = sys.roundTime
	gs.lifeMul = sys.lifeMul
	gs.team1VS2Life = sys.team1VS2Life
	gs.turnsRecoveryRate = sys.turnsRecoveryRate

	gs.changeStateNest = sys.changeStateNest

	gs.drawc1 = make(ClsnRect, len(sys.drawc1))
	copy(gs.drawc1, sys.drawc1)
	gs.drawc2 = make(ClsnRect, len(sys.drawc2))
	copy(gs.drawc2, sys.drawc2)
	gs.drawc2sp = make(ClsnRect, len(sys.drawc2sp))
	copy(gs.drawc2sp, sys.drawc2sp)
	gs.drawc2mtk = make(ClsnRect, len(sys.drawc2mtk))
	copy(gs.drawc2mtk, sys.drawc2mtk)
	gs.drawwh = make(ClsnRect, len(sys.drawwh))
	copy(gs.drawwh, sys.drawwh)

	gs.accel = sys.accel
	gs.clsnDraw = sys.clsnDraw
	gs.statusDraw = sys.statusDraw
	gs.explodMax = sys.explodMax
	gs.workpal = make([]uint32, len(sys.workpal))
	copy(gs.workpal, sys.workpal)
	gs.playerProjectileMax = sys.playerProjectileMax
	gs.nomusic = sys.nomusic
	gs.lifeShare = sys.lifeShare

	gs.turbo = sys.turbo
	gs.drawScale = sys.drawScale
	gs.zoomlag = sys.zoomlag
	gs.zoomScale = sys.zoomScale
	gs.zoomPosXLag = sys.zoomPosXLag
	gs.zoomPosYLag = sys.zoomPosYLag
	gs.enableZoomstate = sys.enableZoomstate
	gs.zoomCameraBound = sys.zoomCameraBound
	gs.zoomPos = sys.zoomPos

	gs.reloadCharSlot = sys.reloadCharSlot
	gs.turbo = sys.turbo
	gs.drawScale = sys.drawScale
	gs.zoomlag = sys.zoomlag
	gs.zoomScale = sys.zoomScale
	gs.zoomPosXLag = sys.zoomPosXLag

	gs.matchWins = sys.matchWins
	gs.wins = sys.wins
	gs.roundsExisted = sys.roundsExisted
	gs.draws = sys.draws
	gs.tmode = sys.tmode
	gs.numSimul = sys.numSimul
	gs.numTurns = sys.numTurns
	gs.esc = sys.esc
	gs.afterImageMax = sys.afterImageMax
	gs.comboExtraFrameWindow = sys.comboExtraFrameWindow
	gs.envcol_under = sys.envcol_under
	gs.helperMax = sys.helperMax
	gs.nextCharId = sys.nextCharId
	gs.powerShare = sys.powerShare
	gs.tickCount = sys.tickCount
	gs.oldTickCount = sys.oldTickCount
	gs.tickCountF = sys.tickCountF
	gs.lastTick = sys.lastTick
	gs.nextAddTime = sys.nextAddTime
	gs.oldNextAddTime = sys.oldNextAddTime
	gs.screenleft = sys.screenleft
	gs.screenright = sys.screenright
	gs.xmin = sys.xmin
	gs.xmax = sys.xmax
	gs.winskipped = sys.winskipped
	gs.paused = sys.paused
	gs.step = sys.step
	gs.roundResetFlg = sys.roundResetFlg
	gs.reloadFlg = sys.reloadFlg
	gs.reloadStageFlg = sys.reloadStageFlg
	gs.reloadLifebarFlg = sys.reloadLifebarFlg

	gs.match = sys.match
	gs.round = sys.round

	if sys.workingState != nil {
		gs.workingStateState = *sys.workingState
	}

}

func (gs *GameState) savePalFX() {
	gs.allPalFX = sys.allPalFX
	gs.bgPalFX = sys.bgPalFX
}

func (gs *GameState) saveCharData() {
	for i := range sys.chars {
		gs.charState[i] = make([]CharState, len(sys.chars[i]))
		for j, c := range sys.chars[i] {
			gs.charState[i][j] = c.getCharState()
			gs.charMap[gs.charState[i][j].id] = gs.charState[i][j]
		}
	}

	if sys.workingChar != nil {
		gs.workingCharState = sys.workingChar.getCharState()
	}
}

func (gs *GameState) saveProjectileData() {
	for i := range sys.projs {
		gs.projectileState[i] = make([]ProjectileState, len(sys.projs[i]))
		for j := 0; j < len(sys.projs[i]); j++ {
			gs.projectileState[i][j] = sys.projs[i][j].getProjectileState()
			gs.projMap[gs.projectileState[i][j].id] = gs.projectileState[i][j]
		}
	}
}

func (gs *GameState) saveSuperData() {
	gs.super = sys.super
	gs.supertime = sys.supertime
	gs.superpausebg = sys.superpausebg
	gs.superendcmdbuftime = sys.superendcmdbuftime
	gs.superplayer = sys.superplayer
	gs.superdarken = sys.superdarken
	if sys.superanim != nil {
		gs.superanimRef = sys.superanim
		gs.superanim = sys.superanim.getAnimationState()
	}
	gs.superpmap = sys.superpmap
	gs.superpos = [2]float32{sys.superpos[0], sys.superpos[1]}
	gs.superfacing = sys.superfacing
	gs.superp2defmul = sys.superp2defmul
}

func (gs *GameState) savePauseData() {
	gs.pause = sys.pause
	gs.pausetime = sys.pausetime
	gs.pausebg = sys.pausebg
	gs.pauseendcmdbuftime = sys.pauseendcmdbuftime
	gs.pauseplayer = sys.pauseplayer
}

func (gs *GameState) saveExplodData() {
	for i := range sys.explods {
		gs.explodsState[i] = make([]ExplodState, len(sys.explods[i]))
		for j := 0; j < len(sys.explods[i]); j++ {
			gs.explodsState[i][j] = sys.explods[i][j].getExplodState()
		}
	}
	for i := range sys.explDrawlist {
		gs.explDrawlist[i] = make([]int, len(sys.explDrawlist[i]))
		copy(gs.explDrawlist[i], sys.explDrawlist[i])
	}

	for i := range sys.topexplDrawlist {
		gs.topexplDrawlist[i] = make([]int, len(sys.topexplDrawlist[i]))
		copy(gs.topexplDrawlist[i], sys.topexplDrawlist[i])
	}

	for i := range sys.underexplDrawlist {
		gs.underexplDrawlist[i] = make([]int, len(sys.underexplDrawlist[i]))
		copy(gs.underexplDrawlist[i], sys.underexplDrawlist[i])
	}
}

func (gs *GameState) loadPalFX() {
	sys.allPalFX = gs.allPalFX
	sys.bgPalFX = gs.bgPalFX
}

func (gs *GameState) charsPersist() bool {
	for i := 0; i < len(sys.chars); i++ {
		if len(sys.chars[i]) != len(gs.charState[i]) {
			return false
		}
		for j := 0; j < len(sys.chars[i]); j++ {
			if sys.chars[i][j].id != gs.charState[i][j].id {
				return false
			}
		}
	}
	return true
}

func (gs *GameState) loadCharData() {
	if gs.charsPersist() {
		fmt.Println("Chars persist")
		for i := range sys.chars {
			for j, _ := range sys.chars[i] {
				sys.chars[i][j].loadCharState(gs.charState[i][j])
			}
		}
	} else {
		fmt.Println("Chars did not persist.")
		/*
			for i := range sys.chars {
				for j, _ := range sys.chars[i] {
					id := sys.chars[i][j].id
					state, ok := gs.charMap[id]
					if ok {
						sys.chars[i][j].loadCharState(state)
					}
				}
			}*/

		for i := range sys.chars {
			fmt.Printf("len of chars %d len of charState %d\n", len(sys.chars[i]), len(gs.charState[i]))
			if len(sys.chars[i]) < len(gs.charState[i]) {
				for len(sys.chars[i]) < len(gs.charState[i]) {
					sys.chars[i][0].newHelper()
				}
			} else if len(sys.chars[i]) > len(gs.charState[i]) {
				for len(sys.chars[i]) > len(gs.charState[i]) {
					sys.chars[i] = sys.chars[i][:len(sys.chars[i])-1]
				}
			}
		}

		for i := range sys.chars {
			for j, _ := range sys.chars[i] {
				sys.chars[i][j].loadCharState(gs.charState[i][j])
			}
		}
	}

	wc := gs.workingCharState.findChar()
	if wc == nil {
		wc = &Char{}
	}
	sys.workingChar = wc
	sys.workingChar.loadCharState(gs.workingCharState)
}

func (gs *GameState) loadSuperData() {
	sys.super = gs.super
	sys.supertime = gs.supertime
	sys.superpausebg = gs.superpausebg
	sys.superendcmdbuftime = gs.superendcmdbuftime
	sys.superplayer = gs.superplayer
	sys.superdarken = gs.superdarken
	if sys.superanim != nil {
		sys.superanim = gs.superanimRef
		if gs.superanimRef != nil {
			sys.superanim.loadAnimationState(gs.superanim)
		}
	}
	sys.superpmap = gs.superpmap
	sys.superpos = [2]float32{gs.superpos[0], gs.superpos[1]}
	sys.superfacing = gs.superfacing
	sys.superp2defmul = gs.superp2defmul
}

func (gs *GameState) loadPauseData() {
	sys.pause = gs.pause
	sys.pausetime = gs.pausetime
	sys.pausebg = gs.pausebg
	sys.pauseendcmdbuftime = gs.pauseendcmdbuftime
	sys.pauseplayer = gs.pauseplayer
}

func (gs *GameState) loadExplodData() {
	for i := range gs.explodsState {
		for j, _ := range gs.explodsState[i] {
			sys.explods[i][j].loadExplodState(gs.explodsState[i][j])
		}
	}
	for i := range gs.explDrawlist {
		sys.explDrawlist[i] = make([]int, len(gs.explDrawlist[i]))
		copy(sys.explDrawlist[i], gs.explDrawlist[i])
	}

	for i := range sys.topexplDrawlist {
		sys.topexplDrawlist[i] = make([]int, len(gs.topexplDrawlist[i]))
		copy(sys.topexplDrawlist[i], gs.topexplDrawlist[i])
	}

	for i := range sys.underexplDrawlist {
		sys.underexplDrawlist[i] = make([]int, len(gs.underexplDrawlist[i]))
		copy(sys.underexplDrawlist[i], gs.underexplDrawlist[i])
	}
}

func (gs *GameState) projectliesPersist() bool {
	for i := 0; i < len(sys.projs); i++ {
		if len(sys.projs[i]) != len(gs.projectileState[i]) {
			return false
		}
		for j := 0; j < len(sys.projs[i]); j++ {
			if sys.projs[i][j].id != gs.projectileState[i][j].id {
				return false
			}
		}
	}
	return true
}

func (gs *GameState) loadProjectileData() {
	if gs.projectliesPersist() {
		fmt.Println("Projectiles Persist")
		for i := range sys.projs {
			for j := range sys.projs[i] {
				sys.projs[i][j].loadProjectileState(gs.projectileState[i][j])
			}
		}
	} else {
		fmt.Println("Projectiles did not persist")
		// for i := range sys.projs {
		// 	fmt.Printf("len of projs %d len of projsState %d\n", len(sys.projs[i]), len(gs.projectileState[i]))
		// 	if len(sys.projs[i]) < len(gs.projectileState[i]) {
		// 		for len(sys.projs[i]) < len(gs.projectileState[i]) {
		// 			sys.chars[i][0].newProj()
		// 		}
		// 	} else if len(sys.projs[i]) > len(gs.projectileState[i]) {
		// 		for len(sys.projs[i]) > len(gs.projectileState[i]) {
		// 			sys.projs[i] = sys.projs[i][:len(sys.projs[i])-1]
		// 		}
		// 	}
		// }
		/*
			for i := range sys.projs {
				for j, _ := range sys.projs[i] {
					id := sys.projs[i][j].id
					state, ok := gs.projMap[id]
					if ok {
						sys.projs[i][j].loadProjectileState(state)
					}
				}
			}*/

		// for i := range sys.projs {
		// 	for j := range sys.projs[i] {
		// 		sys.projs[i][j].loadProjectileState(gs.projectileState[i][j])
		// 	}
		// }
		// for i := range sys.projs {
		// 	for j := range sys.projs[i] {
		// 		sys.projs[i][j].loadProjectileState(gs.projectileState[i][j])
		// 	}
		// }
		for i := range gs.projectileState {
			sys.projs[i] = make([]Projectile, len(gs.projectileState[i]))
			for j := range gs.projectileState[i] {
				sys.projs[i][j] = *gs.projectileState[i][j].ptr
				sys.projs[i][j].loadProjectileState(gs.projectileState[i][j])
			}
		}

	}

}
