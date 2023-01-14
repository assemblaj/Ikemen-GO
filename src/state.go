package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"strconv"
	"sync"
	"time"

	"arena"

	glfw "github.com/fyne-io/glfw-js"
	lua "github.com/yuin/gopher-lua"
)

func init() {

	sys.gameState.randseed = sys.randseed

	for i := 0; i < 8; i++ {
		gs := NewGameState()
		sys.gameStatePool <- gs
		rs := NewReplayState()
		sys.replayPool <- &rs
	}

	for i := 0; i < 8; i++ {
		sys.gameStates[i] = *NewGameState()
	}
	gob.Register(GameState{})
	gob.Register(CharState{})

}

type ReplayState struct {
	id         int
	state      *bytes.Buffer
	ib         [MaxSimul*2 + MaxAttachedChar]InputBits
	buf        [MaxSimul*2 + MaxAttachedChar]NetBuffer
	delay      int32
	locIn      int
	remIn      int
	time       int32
	stoppedcnt int32
	startState GameState
	endState   []GameState
	syncTest   bool
	replayEnd  bool
}

func (rs *ReplayState) RecordInput(cb *CommandBuffer, i int, facing int32) {
	//fmt.Println("record Input")
	if i >= 0 && i < len(rs.buf) {
		rs.buf[sys.inputRemap[i]].input(cb, facing)
	}
}

func (rs *ReplayState) PlayInput(cb *CommandBuffer, i int, facing int32) {
	//fmt.Println("play Input")
	if i >= 0 && i < len(rs.ib) {
		rs.ib[sys.inputRemap[i]].GetInput(cb, facing)
	}
}

func (rs *ReplayState) PlayAnyButton() bool {
	for _, b := range rs.ib {
		if b&IB_anybutton != 0 {
			return true
		}
	}
	return false
}
func (rs *ReplayState) RecordUpdate() bool {
	if !sys.gameEnd {
		rs.buf[rs.locIn].localUpdate(0)
		if rs.state != nil {
			for _, nb := range rs.buf {
				binary.Write(rs.state, binary.LittleEndian, &nb.buf[rs.time&31])
			}
		}
		rs.time++
	}
	return !sys.gameEnd
}

func (rs *ReplayState) RecordAnyButton() bool {
	//fmt.Println("RecordAnyButton")
	for _, nb := range rs.buf {
		if nb.buf[nb.curT&31]&IB_anybutton != 0 {
			return true
		}
	}
	return false
}

func (rs *ReplayState) getLastFrame() (GameState, bool) {
	for i := range rs.endState {
		if rs.endState[i].GameTime == rs.startState.GameTime {
			return rs.endState[i], true
		}
	}
	return GameState{}, false
}
func (rs *ReplayState) PlayUpdate() bool {
	//fmt.Println("PlayUpdate")
	if rs.syncTest {
		sys.gameState.SaveState(0)
		rs.endState = append(rs.endState, sys.gameState)
	}
	// now := sys.frameCounter
	// if now-int32(sys.gameState.rollback.rollbackTimer) == sys.gameState.rollback.rollbackWindow {
	// 	rs.replayEnd = true
	// }

	if sys.oldNextAddTime > 0 &&
		binary.Read(rs.state, binary.LittleEndian, rs.ib[:]) != nil {
		sys.playReplayState = false
		rs.replayEnd = true
		if rs.syncTest {
			frame, ok := rs.getLastFrame()
			if !ok {
				fmt.Println("State not found.")
			} else {
				if !rs.startState.Equal(frame) {
					panic("SyncError.")
				} else {
					fmt.Println("Sync Test Passed.")
				}
			}
		}
	}
	return !sys.gameEnd
}
func (rs *ReplayState) getID() string {
	return strconv.Itoa(rs.id)
}

func NewReplayState() ReplayState {
	return ReplayState{
		state: bytes.NewBuffer([]byte{}),
		id:    int(time.Now().UnixMilli()),
	}
}

type CharState struct {
	palFX                 PalFX
	ChildrenState         []CharState
	EnemynearState        [2][]CharState
	curFramePtr           *AnimFrame
	curFrame              AnimFrame
	animState             AnimationState
	cmd                   []CommandList
	ss                    StateState
	hitdef                HitDef
	RedLife               int32
	Juggle                int32
	Life                  int32
	Name                  string
	Key                   int
	Id                    int32
	HelperId              int32
	HelperIndex           int32
	ParentIndex           int32
	PlayerNo              int
	Teamside              int
	player                bool
	AnimPN                int
	AnimNo                int32
	LifeMax               int32
	PowerMax              int32
	DizzyPoints           int32
	dizzyPointsMax        int32
	GuardPoints           int32
	guardPointsMax        int32
	FallTime              int32
	Localcoord            float32
	Localscl              float32
	ClsnScale             [2]float32
	HoIdx                 int
	Mctime                int32
	Targets               []int32
	TargetsOfHitdef       []int32
	Pos                   [3]float32
	DrawPos               [3]float32
	OldPos                [3]float32
	Vel                   [3]float32
	Facing                float32
	csv                   CharSystemVar
	p1facing              float32
	cpucmd                int32
	attackDist            float32
	stchtmp               bool
	inguarddist           bool
	pushed                bool
	hitdefContact         bool
	Atktmp                int8
	Hittmp                int8
	Acttmp                int8
	Minus                 int8
	platformPosY          float32
	GroundAngle           float32
	ownpal                bool
	winquote              int32
	memberNo              int
	selectNo              int
	ComboExtraFrameWindow int32
	InheritJuggle         int32
	immortal              bool
	kovelocity            bool
	Preserve              int32
	inputFlag             InputBits
	pauseBool             bool

	keyctrl         [4]bool
	power           int32
	size            CharSize
	ghv             GetHitVar
	hitby           [2]HitBy
	ho              [8]HitOverride
	mctype          MoveContact
	Ivar            [NumVar + NumSysVar]int32
	Fvar            [NumFvar + NumSysFvar]float32
	Offset          [2]float32
	mapArray        map[string]float32
	mapDefault      map[string]float32
	remapSpr        RemapPreset
	clipboardText   []string
	dialogue        []string
	defaultHitScale [3]*HitScale
	nextHitScale    map[int32][3]*HitScale
	activeHitScale  map[int32][3]*HitScale
}

func (cs *CharState) String() string {
	str := fmt.Sprintf(`Char %s 
	RedLife             :%d 
	Juggle              :%d 
	Life                :%d 
	Key                 :%d  
	Localcoord          :%f 
	Localscl            :%f 
	Pos                 :%v 
	DrawPos             :%v 
	OldPos              :%v 
	Vel                 :%v  
	Facing              :%f
	Id                  :%d
	HelperId            :%d
	HelperIndex         :%d
	ParentIndex         :%d
	PlayerNo            :%d
	Teamside            :%d
	AnimPN              :%d
	AnimNo              :%d
	LifeMax             :%d
	PowerMax            :%d
	DizzyPoints         :%d
	GuardPoints         :%d
	FallTime            :%d
	ClsnScale           :%v
	HoIdx               :%d
	Mctime              :%d
	Targets             :%v
	TargetsOfHitdef     :%v
	Atktmp              :%d
	Hittmp              :%d
	Acttmp              :%d
	Minus               :%d
	GroundAngle          :%f
	ComboExtraFrameWindow :%d
	InheritJuggle         :%d
	Preserve              :%d
	Ivar            :%v
	Fvar            :%v
	Offset          :%v`,
		cs.Name, cs.RedLife, cs.Juggle, cs.Life, cs.Key, cs.Localcoord,
		cs.Localscl, cs.Pos, cs.DrawPos, cs.OldPos, cs.Vel, cs.Facing,
		cs.Id, cs.HelperId, cs.HelperIndex, cs.ParentIndex, cs.PlayerNo,
		cs.Teamside, cs.AnimPN, cs.AnimNo, cs.LifeMax, cs.PowerMax, cs.DizzyPoints,
		cs.GuardPoints, cs.FallTime, cs.ClsnScale, cs.HoIdx, cs.Mctime, cs.Targets, cs.TargetsOfHitdef,
		cs.Atktmp, cs.Hittmp, cs.Acttmp, cs.Minus, cs.GroundAngle, cs.ComboExtraFrameWindow, cs.InheritJuggle,
		cs.Preserve, cs.Ivar, cs.Fvar, cs.Offset)
	str += fmt.Sprintf("\nChildren of %s:", cs.Name)
	if len(cs.ChildrenState) == 0 {
		str += "None\n"
	} else {
		str += "{ \n"
		for i := 0; i < len(cs.ChildrenState); i++ {
			str += cs.ChildrenState[i].String()
			str += "\n"
		}
		str += "}\n"

	}
	str += fmt.Sprintf("EnemyNear of %s:", cs.Name)
	if len(cs.EnemynearState[0]) == 0 && len(cs.EnemynearState[1]) == 0 {
		str += "None\n"
	} else {
		str += "{ \n "
		for i := 0; i < len(cs.EnemynearState); i++ {
			for j := 0; j < len(cs.EnemynearState[i]); j++ {
				str += cs.EnemynearState[i][j].String()
				str += "\n"
			}
		}
		str += "}\n"

	}
	return str
}

func (cs Char) String() string {
	str := fmt.Sprintf(`Char %s 
	RedLife             :%d 
	Juggle              :%d 
	Life                :%d 
	Key                 :%d  
	Localcoord          :%f 
	Localscl            :%f 
	Pos                 :%v 
	DrawPos             :%v 
	OldPos              :%v 
	Vel                 :%v  
	Facing              :%f
	Id                  :%d
	HelperId            :%d
	HelperIndex         :%d
	ParentIndex         :%d
	PlayerNo            :%d
	Teamside            :%d
	AnimPN              :%d
	AnimNo              :%d
	LifeMax             :%d
	PowerMax            :%d
	DizzyPoints         :%d
	GuardPoints         :%d
	FallTime            :%d
	ClsnScale           :%v
	HoIdx               :%d
	Mctime              :%d
	Targets             :%v
	TargetsOfHitdef     :%v
	Atktmp              :%d
	Hittmp              :%d
	Acttmp              :%d
	Minus               :%d
	GroundAngle          :%f
	ComboExtraFrameWindow :%d
	InheritJuggle         :%d
	Preserve              :%d
	Ivar            :%v
	Fvar            :%v
	Offset          :%v`,
		cs.name, cs.redLife, cs.juggle, cs.life, cs.key, cs.localcoord,
		cs.localscl, cs.pos, cs.drawPos, cs.oldPos, cs.vel, cs.facing,
		cs.id, cs.helperId, cs.helperIndex, cs.parentIndex, cs.playerNo,
		cs.teamside, cs.animPN, cs.animNo, cs.lifeMax, cs.powerMax, cs.dizzyPoints,
		cs.guardPoints, cs.fallTime, cs.clsnScale, cs.hoIdx, cs.mctime, cs.targets, cs.targetsOfHitdef,
		cs.atktmp, cs.hittmp, cs.acttmp, cs.minus, cs.groundAngle, cs.comboExtraFrameWindow, cs.inheritJuggle,
		cs.preserve, cs.ivar, cs.fvar, cs.offset)
	str += fmt.Sprintf("\nChildren of %s:", cs.name)
	if len(cs.children) == 0 {
		str += "None\n"
	} else {
		str += "{ \n"
		for i := 0; i < len(cs.children); i++ {
			if cs.children[i] != nil {
				str += cs.children[i].String()
			} else {
				str += "Nil Child"
			}
			str += "\n"
		}
		str += "}\n"

	}
	str += fmt.Sprintf("EnemyNear of %s:", cs.name)
	if len(cs.enemynear[0]) == 0 && len(cs.enemynear[1]) == 0 {
		str += "None\n"
	} else {
		str += "{ \n "
		for i := 0; i < len(cs.enemynear); i++ {
			for j := 0; j < len(cs.enemynear[i]); j++ {
				str += cs.enemynear[i][j].String()
				str += "\n"
			}
		}
		str += "}\n"

	}
	return str
}

func (cs *CharState) findChar() *Char {
	for i := 0; i < len(sys.chars); i++ {
		for j := 0; j < len(sys.chars[i]); j++ {
			if cs.Id == sys.chars[i][j].id {
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

type RollbackState struct {
	rollbackTest   bool
	rollbackTimer  int32
	rollbackWindow int32
	loaded         bool
	saved          bool
	justSaved      bool
	playingReplay  bool
	flag           bool
}

func (rs RollbackState) findLastFrame(cur int32) (*GameState, int) {
	var toReturn *GameState = nil
	var returnIdx int = -1

	last := cur - rs.rollbackWindow + 1
	fmt.Printf("Last %d \n", last)
	for i := 0; i < len(sys.gameStates); i++ {
		fmt.Printf("I'm here in findLastFrame %d\n", sys.gameStates[i].frame)
		if sys.gameStates[i].frame == last {
			fmt.Println("Sys.gameStates")
			toReturn = &sys.gameStates[i]
			returnIdx = i
		} else {
			if sys.gameStates[i].frame < last+rs.rollbackWindow {
				fmt.Printf("sys.gameStates[i].frame % d< last+rs.rollbackWindow %d\n", sys.gameStates[i].frame,
					last+rs.rollbackWindow)

				// 	fmt.Println("Trapped in here? ")
				select {
				case sys.gameStatePool <- &sys.gameStates[i]:
				default:
					fmt.Println("GameState channel full")
				}
				select {
				case sys.replayPool <- &sys.replays[i]:
				default:
					fmt.Println("Replay channel full")
				}
			}
		}
	}
	return toReturn, returnIdx
}

func (gs *GameState) getID() string {
	return strconv.Itoa(int(gs.id))
}

func (gs *GameState) Checksum() int {
	//	buf := bytes.Buffer{}
	//	enc := gob.NewEncoder(&buf)
	//	err := enc.Encode(gs)
	//	if err != nil {
	//		panic(err)
	//	}
	//	gs.bytes = buf.Bytes()
	gs.bytes = []byte(gs.String())
	h := fnv.New32a()
	h.Write(gs.bytes)
	return int(h.Sum32())
}

func (gs *GameState) String() (str string) {
	str = fmt.Sprintf("Time: %d GameTime %d \n", gs.Time, gs.GameTime)
	str += fmt.Sprintf("bcStack: %v\n", gs.bcStack)
	str += fmt.Sprintf("bcVarStack: %v\n", gs.bcVarStack)
	str += fmt.Sprintf("bcVar: %v\n", gs.bcVar)
	str += fmt.Sprintf("workBe: %v\n", gs.workBe)
	for i := 0; i < len(gs.charData); i++ {
		for j := 0; j < len(gs.charData[i]); j++ {
			str += gs.charData[i][j].String()
			str += "\n"
		}
	}
	return
}

type CharListState struct {
	runOrder, drawOrder       []CharState
	runOrderPtr, drawOrderPtr []*Char
	idMap                     map[int32]CharState
	idMapPtr                  map[int32]*Char
}

const MaxSaveStates = 8

type GameState struct {
	bytes             []byte
	id                int
	saved             bool
	rollback          RollbackState
	frame             int32
	randseed          int32
	Time              int32
	GameTime          int32
	projs             [MaxSimul*2 + MaxAttachedChar][]Projectile
	CharState         [MaxSimul*2 + MaxAttachedChar][]CharState
	chars             [MaxSimul*2 + MaxAttachedChar][]*Char
	charData          [MaxSimul*2 + MaxAttachedChar][]Char
	explods           [MaxSimul*2 + MaxAttachedChar][]Explod
	explDrawlist      [MaxSimul*2 + MaxAttachedChar][]int
	topexplDrawlist   [MaxSimul*2 + MaxAttachedChar][]int
	underexplDrawlist [MaxSimul*2 + MaxAttachedChar][]int
	aiInput           [MaxSimul*2 + MaxAttachedChar]AiInput
	inputRemap        [MaxSimul*2 + MaxAttachedChar]int
	autoguard         [MaxSimul*2 + MaxAttachedChar]bool
	charList          CharList
	charMap           map[int32]CharState
	projMap           map[int32]ProjectileState

	com                [MaxSimul*2 + MaxAttachedChar]float32 // UIT
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
	superanim          *Animation
	superanimRef       *Animation
	superpmap          PalFX
	superpos           [2]float32
	superfacing        float32
	superp2defmul      float32

	envShake            EnvShake
	specialFlag         GlobalSpecialFlag // UIT
	envcol              [3]int32
	envcol_time         int32
	bcStack, bcVarStack BytecodeStack
	bcVar               []BytecodeValue
	stageState          StageState
	workBe              []BytecodeExp

	scrrect                 [4]int32
	gameWidth, gameHeight   int32 // UIT
	widthScale, heightScale float32
	gameEnd, frameSkip      bool
	brightness              int32
	roundTime               int32 // UIT
	lifeMul                 float32
	team1VS2Life            float32
	turnsRecoveryRate       float32
	match                   int32 // UIT
	round                   int32 // UIT
	intro                   int32
	lastHitter              [2]int
	winTeam                 int // UIT
	winType                 [2]WinType
	winTrigger              [2]WinType // UIT
	matchWins, wins         [2]int32   // UIT
	roundsExisted           [2]int32
	draws                   int32
	tmode                   [2]TeamMode // UIT
	numSimul, numTurns      [2]int32    // UIT
	esc                     bool
	workingChar             *Char
	workingCharState        CharState
	workingStateState       StateBytecode // UIT
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
	finish                  FinishType // UIT
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
	keyConfig               []KeyConfig
	joystickConfig          []KeyConfig
	lifebar                 Lifebar
	redrawWait              struct{ nextTime, lastDraw time.Time }
	cgi                     [MaxSimul*2 + MaxAttachedChar]CharGlobalInfo

	// New 11/04/2022 all UIT
	timerStart      int32
	timerRounds     []int32
	teamLeader      [2]int
	stage           *Stage
	postMatchFlg    bool
	scoreStart      [2]float32
	scoreRounds     [][2]float32
	roundType       [2]RoundType
	sel             Select
	stringPool      [MaxSimul*2 + MaxAttachedChar]StringPool
	dialogueFlg     bool
	gameMode        string
	consecutiveWins [2]int32
	home            int

	// Non UIT, but adding them anyway just because
	// Used in Stage.go
	stageLoop bool

	// Sound
	panningRange  float32
	stereoEffects bool
	bgmVolume     int
	audioDucking  bool
	wavVolume     int

	// ByteCode
	dialogueBarsFlg bool
	dialogueForce   int
	playBgmFlg      bool

	// Input
	keyInput  glfw.Key
	keyString string

	// LifeBar
	timerCount []int32

	// Script
	commonLua    []string
	commonStates []string
	endMatch     bool
	matchData    *lua.LTable
	noSoundFlg   bool
	loseSimul    bool
	loseTag      bool
	continueFlg  bool

	stageLoopNo int

	// 11/5/2022
	fight Fight

	debugWC *Char

	commandLists []*CommandList
	luaTables    []*lua.LTable
	arena        *arena.Arena
}

func NewGameState() *GameState {
	return &GameState{
		id: int(time.Now().UnixMilli()),
	}
}

func (gs *GameState) Equal(other GameState) (equality bool) {

	if gs.randseed != other.randseed {
		fmt.Printf("Error on randseed: %d : %d ", gs.randseed, other.randseed)
		return false
	}

	if gs.Time != other.Time {
		fmt.Println("Error on time.")
		return false
	}

	if gs.GameTime != other.GameTime {
		fmt.Println("Error on gameTime.")
		return false
	}
	return true

}

func (gs *GameState) LoadState(stateID int) {
	sys.arenaLoadMap[stateID] = arena.NewArena()
	a := sys.arenaLoadMap[stateID]

	sys.randseed = gs.randseed
	sys.time = gs.Time // UIT
	sys.gameTime = gs.GameTime
	gs.loadCharData(a)
	gs.loadExplodData(a)
	sys.cam = gs.cam
	gs.loadPauseData()
	gs.loadSuperData(a)
	gs.loadPalFX(a)
	gs.loadProjectileData(a)
	sys.com = gs.com
	sys.envShake = gs.envShake
	sys.envcol_time = gs.envcol_time
	sys.specialFlag = gs.specialFlag
	sys.envcol = gs.envcol

	sys.bcStack = arena.MakeSlice[BytecodeValue](a, len(gs.bcStack), len(gs.bcStack))
	copy(sys.bcStack, gs.bcStack)
	sys.bcVarStack = arena.MakeSlice[BytecodeValue](a, len(gs.bcVarStack), len(gs.bcVarStack))
	copy(sys.bcVarStack, gs.bcVarStack)
	sys.bcVar = arena.MakeSlice[BytecodeValue](a, len(gs.bcVar), len(gs.bcVar))
	copy(sys.bcVar, gs.bcVar)

	sys.stage = gs.stage.clone(a)

	sys.aiInput = gs.aiInput
	sys.inputRemap = gs.inputRemap
	sys.autoguard = gs.autoguard

	sys.workBe = arena.MakeSlice[BytecodeExp](a, len(gs.workBe), len(gs.workBe))
	for i := 0; i < len(gs.workBe); i++ {
		sys.workBe[i] = arena.MakeSlice[OpCode](a, len(gs.workBe[i]), len(gs.workBe[i]))
		copy(sys.workBe[i], gs.workBe[i])
	}

	sys.finish = gs.finish
	sys.winTeam = gs.winTeam
	sys.winType = gs.winType
	sys.winTrigger = gs.winTrigger
	sys.lastHitter = gs.lastHitter
	sys.waitdown = gs.waitdown
	sys.slowtime = gs.slowtime

	sys.shuttertime = gs.shuttertime
	//sys.fadeintime = gs.fadeintime
	//sys.fadeouttime = gs.fadeouttime
	sys.winskipped = gs.winskipped

	sys.intro = gs.intro
	sys.time = gs.Time
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

	sys.drawc1 = arena.MakeSlice[[4]float32](a, len(gs.drawc1), len(gs.drawc1))
	copy(sys.drawc1, gs.drawc1)
	sys.drawc2 = arena.MakeSlice[[4]float32](a, len(gs.drawc2), len(gs.drawc2))
	copy(sys.drawc2, gs.drawc2)
	sys.drawc2sp = arena.MakeSlice[[4]float32](a, len(gs.drawc2sp), len(gs.drawc2sp))
	copy(sys.drawc2sp, gs.drawc2sp)
	sys.drawc2mtk = arena.MakeSlice[[4]float32](a, len(gs.drawc2mtk), len(gs.drawc2mtk))
	copy(sys.drawc2mtk, gs.drawc2mtk)
	sys.drawwh = arena.MakeSlice[[4]float32](a, len(gs.drawwh), len(gs.drawwh))
	copy(sys.drawwh, gs.drawwh)

	sys.accel = gs.accel
	sys.clsnDraw = gs.clsnDraw
	//sys.statusDraw = gs.statusDraw
	sys.explodMax = gs.explodMax

	// Things that directly or indirectly get put into CGO can't go into arenas
	sys.workpal = make([]uint32, len(gs.workpal)) //arena.MakeSlice[uint32](a, len(gs.workpal), len(gs.workpal))
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

	// bug, if a prior state didn't have this
	// Did the prior state actually have a working state
	if gs.workingStateState.stateType != 0 && gs.workingStateState.moveType != 0 {
		// if sys.workingState != nil {
		// 	*sys.workingState = gs.workingStateState
		// } else {
		ws := gs.workingStateState.clone(a)
		sys.workingState = &ws
		// }
	}

	sys.lifebar = gs.lifebar.clone(a)

	sys.cgi = gs.cgi

	sys.timerStart = gs.timerStart

	sys.timerRounds = arena.MakeSlice[int32](a, len(gs.timerRounds), len(gs.timerRounds))
	copy(sys.timerRounds, gs.timerRounds)

	sys.teamLeader = gs.teamLeader
	sys.postMatchFlg = gs.postMatchFlg
	sys.scoreStart = gs.scoreStart

	sys.scoreRounds = arena.MakeSlice[[2]float32](a, len(gs.scoreRounds), len(gs.scoreRounds))
	copy(sys.scoreRounds, gs.scoreRounds)

	sys.roundType = gs.roundType

	sys.sel = gs.sel.clone(a)
	for i := 0; i < len(sys.stringPool); i++ {
		sys.stringPool[i] = gs.stringPool[i].clone(a)
	}

	sys.dialogueFlg = gs.dialogueFlg
	sys.gameMode = gs.gameMode
	sys.consecutiveWins = gs.consecutiveWins
	sys.home = gs.home

	// Not UIT
	sys.stageLoop = gs.stageLoop
	sys.panningRange = gs.panningRange
	sys.stereoEffects = gs.stereoEffects
	sys.bgmVolume = gs.bgmVolume
	sys.audioDucking = gs.audioDucking
	sys.wavVolume = gs.wavVolume
	sys.dialogueBarsFlg = gs.dialogueBarsFlg
	sys.dialogueForce = gs.dialogueForce
	sys.playBgmFlg = gs.playBgmFlg
	//sys.keyState = gs.keyState
	sys.keyInput = gs.keyInput
	sys.keyString = gs.keyString

	sys.timerCount = arena.MakeSlice[int32](a, len(gs.timerCount), len(gs.timerCount))
	copy(sys.timerCount, gs.timerCount)
	sys.commonLua = arena.MakeSlice[string](a, len(gs.commonLua), len(gs.commonLua))
	copy(sys.commonLua, gs.commonLua)
	sys.commonStates = arena.MakeSlice[string](a, len(gs.commonStates), len(gs.commonStates))
	copy(sys.commonStates, gs.commonStates)

	sys.endMatch = gs.endMatch

	// theoretically this shouldn't do anything.
	sys.matchData = gs.cloneLuaTable(gs.matchData)

	sys.noSoundFlg = gs.noSoundFlg
	sys.loseSimul = gs.loseSimul
	sys.loseTag = gs.loseTag
	sys.continueFlg = gs.continueFlg
	sys.stageLoopNo = gs.stageLoopNo

	// 11/5/22
	sys.currentFight = gs.fight.clone(sys.arenaLoadMap[stateID])

	wc := gs.debugWC.clone(sys.arenaLoadMap[stateID])
	sys.debugWC = &wc

	// sys.commandLists = gs.commandLists
	// sys.luaTables = gs.luaTables
}

func (gs *GameState) SaveState(stateID int) {
	sys.arenaSaveMap[stateID] = arena.NewArena()
	a := sys.arenaSaveMap[stateID]

	gs.cgi = sys.cgi
	gs.saved = true
	gs.frame = sys.frameCounter
	gs.randseed = sys.randseed
	gs.Time = sys.time
	gs.GameTime = sys.gameTime

	gs.saveCharData(a)
	gs.saveExplodData(a)
	gs.cam = sys.cam
	gs.savePauseData()
	gs.saveSuperData(a)
	gs.savePalFX(a)
	gs.saveProjectileData(a)

	gs.com = sys.com
	gs.envShake = sys.envShake
	gs.envcol_time = sys.envcol_time
	gs.specialFlag = sys.specialFlag
	gs.envcol = sys.envcol

	gs.bcStack = arena.MakeSlice[BytecodeValue](a, len(sys.bcStack), len(sys.bcStack))
	copy(gs.bcStack, sys.bcStack)
	gs.bcVarStack = arena.MakeSlice[BytecodeValue](a, len(sys.bcVarStack), len(sys.bcVarStack))
	copy(gs.bcVarStack, sys.bcVarStack)
	gs.bcVar = arena.MakeSlice[BytecodeValue](a, len(sys.bcVar), len(sys.bcVar))
	copy(gs.bcVar, sys.bcVar)

	gs.stage = sys.stage.clone(a)

	gs.aiInput = sys.aiInput
	gs.inputRemap = sys.inputRemap
	gs.autoguard = sys.autoguard
	gs.workBe = arena.MakeSlice[BytecodeExp](a, len(sys.workBe), len(sys.workBe))
	for i := 0; i < len(sys.workBe); i++ {
		gs.workBe[i] = arena.MakeSlice[OpCode](a, len(sys.workBe[i]), len(sys.workBe[i]))
		copy(gs.workBe[i], sys.workBe[i])
	}

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
	gs.Time = sys.time
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

	gs.drawc1 = arena.MakeSlice[[4]float32](a, len(sys.drawc1), len(sys.drawc1))
	copy(gs.drawc1, sys.drawc1)
	gs.drawc2 = arena.MakeSlice[[4]float32](a, len(sys.drawc2), len(sys.drawc2))
	copy(gs.drawc2, sys.drawc2)
	gs.drawc2sp = arena.MakeSlice[[4]float32](a, len(sys.drawc2sp), len(sys.drawc2sp))
	copy(gs.drawc2sp, sys.drawc2sp)
	gs.drawc2mtk = arena.MakeSlice[[4]float32](a, len(sys.drawc2mtk), len(sys.drawc2mtk))
	copy(gs.drawc2mtk, sys.drawc2mtk)
	gs.drawwh = arena.MakeSlice[[4]float32](a, len(sys.drawwh), len(sys.drawwh))
	copy(gs.drawwh, sys.drawwh)

	gs.accel = sys.accel
	gs.clsnDraw = sys.clsnDraw
	gs.statusDraw = sys.statusDraw
	gs.explodMax = sys.explodMax

	// Things that directly or indirectly get put into CGO can't go into arenas
	gs.workpal = make([]uint32, len(sys.workpal)) //arena.MakeSlice[uint32](a, len(sys.workpal), len(sys.workpal))
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

	// bug, if a prior state didn't have this
	if sys.workingState != nil {
		gs.workingStateState = sys.workingState.clone(a)
	}

	gs.lifebar = sys.lifebar.clone(a)

	gs.timerStart = sys.timerStart
	gs.timerRounds = arena.MakeSlice[int32](a, len(sys.timerRounds), len(sys.timerRounds))
	copy(gs.timerRounds, sys.timerRounds)
	gs.teamLeader = sys.teamLeader
	gs.postMatchFlg = sys.postMatchFlg
	gs.scoreStart = sys.scoreStart
	gs.scoreRounds = arena.MakeSlice[[2]float32](a, len(sys.scoreRounds), len(sys.scoreRounds))
	copy(gs.scoreRounds, sys.scoreRounds)
	gs.roundType = sys.roundType
	gs.sel = sys.sel.clone(a)
	for i := 0; i < len(sys.stringPool); i++ {
		gs.stringPool[i] = sys.stringPool[i].clone(a)
	}

	gs.dialogueFlg = sys.dialogueFlg
	gs.gameMode = sys.gameMode
	gs.consecutiveWins = sys.consecutiveWins

	gs.stageLoop = sys.stageLoop
	gs.panningRange = sys.panningRange
	gs.stereoEffects = sys.stereoEffects
	gs.bgmVolume = sys.bgmVolume
	gs.audioDucking = sys.audioDucking
	gs.wavVolume = sys.wavVolume
	gs.dialogueBarsFlg = sys.dialogueBarsFlg
	gs.dialogueForce = sys.dialogueForce
	gs.playBgmFlg = sys.playBgmFlg

	gs.keyInput = sys.keyInput
	gs.keyString = sys.keyString

	gs.timerCount = arena.MakeSlice[int32](a, len(sys.timerCount), len(sys.timerCount))
	copy(gs.timerCount, sys.timerCount)
	gs.commonLua = arena.MakeSlice[string](a, len(sys.commonLua), len(sys.commonLua))
	copy(gs.commonLua, sys.commonLua)
	gs.commonStates = arena.MakeSlice[string](a, len(sys.commonStates), len(sys.commonStates))
	copy(gs.commonStates, sys.commonStates)

	gs.endMatch = sys.endMatch

	// can't deep copy because its members are private
	matchData := *sys.matchData
	gs.matchData = &matchData

	gs.noSoundFlg = sys.noSoundFlg
	gs.loseSimul = sys.loseSimul
	gs.loseTag = sys.loseTag
	gs.continueFlg = sys.continueFlg
	gs.stageLoopNo = sys.stageLoopNo

	gs.fight = sys.currentFight.clone(a)

	debugWC := sys.debugWC.clone(a)
	gs.debugWC = &debugWC
	gs.commandLists = arena.MakeSlice[*CommandList](a, len(sys.commandLists), len(sys.commandLists))
	for i := 0; i < len(sys.commandLists); i++ {
		cl := sys.commandLists[i].clone(a)
		gs.commandLists[i] = &cl
	}
	gs.luaTables = arena.MakeSlice[*lua.LTable](a, len(sys.luaTables), len(sys.luaTables))
	for i := 0; i < len(sys.luaTables); i++ {
		gs.luaTables[i] = gs.cloneLuaTable(sys.luaTables[i])
	}

}

func (gs *GameState) cloneLuaTable(s *lua.LTable) *lua.LTable {
	tbl := sys.luaLState.NewTable()
	s.ForEach(func(key lua.LValue, value lua.LValue) {
		switch value.Type() {
		case lua.LTTable:
			innerTbl := value.(*lua.LTable)
			tbl.RawSet(key, gs.cloneLuaTable(innerTbl))
		default:
			tbl.RawSet(key, value)
		}
	})
	return tbl
}

func (gs *GameState) savePalFX(a *arena.Arena) {
	gs.allPalFX = sys.allPalFX.clone(a)
	gs.bgPalFX = sys.bgPalFX.clone(a)
}

func (gs *GameState) saveCharData(a *arena.Arena) {
	for i := range sys.chars {
		gs.charData[i] = arena.MakeSlice[Char](a, len(sys.chars[i]), len(sys.chars[i]))
		gs.chars[i] = arena.MakeSlice[*Char](a, len(sys.chars[i]), len(sys.chars[i]))
		for j, c := range sys.chars[i] {
			gs.charData[i][j] = c.clone(a)
			gs.chars[i][j] = c
		}
	}

	for i := range gs.chars {
		for _, c := range gs.chars[i] {
			if !c.keyctrl[0] {
				c.cmd = gs.chars[c.playerNo][0].cmd
			}
		}
	}

	if sys.workingChar != nil {
		c := sys.workingChar.clone(a)
		gs.workingChar = &c
	} else {
		gs.workingChar = sys.workingChar
	}

	gs.charList = sys.charList.clone(a)

}

func (gs *GameState) saveProjectileData(a *arena.Arena) {
	for i := range sys.projs {
		gs.projs[i] = arena.MakeSlice[Projectile](a, len(sys.projs[i]), len(sys.projs[i]))
		for j := 0; j < len(sys.projs[i]); j++ {
			gs.projs[i][j] = sys.projs[i][j].clone(a)
		}
	}
}

func (gs *GameState) saveSuperData(a *arena.Arena) {
	gs.super = sys.super
	gs.supertime = sys.supertime
	gs.superpausebg = sys.superpausebg
	gs.superendcmdbuftime = sys.superendcmdbuftime
	gs.superplayer = sys.superplayer
	gs.superdarken = sys.superdarken
	if sys.superanim != nil {
		gs.superanim = sys.superanim.clone(a)
	} else {
		gs.superanim = sys.superanim
	}
	gs.superpmap = sys.superpmap.clone(a)
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

func (gs *GameState) saveExplodData(a *arena.Arena) {
	for i := range sys.explods {
		gs.explods[i] = arena.MakeSlice[Explod](a, len(sys.explods[i]), len(sys.explods[i]))
		for j := 0; j < len(sys.explods[i]); j++ {
			gs.explods[i][j] = *sys.explods[i][j].clone(a)
		}
	}
	for i := range sys.explDrawlist {
		gs.explDrawlist[i] = arena.MakeSlice[int](a, len(sys.explDrawlist[i]), len(sys.explDrawlist[i]))
		copy(gs.explDrawlist[i], sys.explDrawlist[i])
	}

	for i := range sys.topexplDrawlist {
		gs.topexplDrawlist[i] = arena.MakeSlice[int](a, len(sys.topexplDrawlist[i]), len(sys.topexplDrawlist[i]))
		copy(gs.topexplDrawlist[i], sys.topexplDrawlist[i])
	}

	for i := range sys.underexplDrawlist {
		gs.underexplDrawlist[i] = arena.MakeSlice[int](a, len(sys.underexplDrawlist[i]), len(sys.underexplDrawlist[i]))
		copy(gs.underexplDrawlist[i], sys.underexplDrawlist[i])
	}
}

func (gs *GameState) loadPalFX(a *arena.Arena) {
	sys.allPalFX = gs.allPalFX.clone(a)
	sys.bgPalFX = gs.bgPalFX.clone(a)
}

func (gs *GameState) charsPersist() bool {
	for i := 0; i < len(sys.chars); i++ {
		if len(sys.chars[i]) != len(gs.CharState[i]) {
			return false
		}
		for j := 0; j < len(sys.chars[i]); j++ {
			if sys.chars[i][j].id != gs.CharState[i][j].Id {
				return false
			}
		}
	}
	return true
}

func (gs *GameState) loadCharData(a *arena.Arena) {
	for i := 0; i < len(sys.chars); i++ {
		sys.chars[i] = arena.MakeSlice[*Char](a, len(gs.chars[i]), len(gs.chars[i]))
		copy(sys.chars[i], gs.chars[i])
	}

	for i := 0; i < len(sys.chars); i++ {
		for j := 0; j < len(sys.chars[i]); j++ {
			*sys.chars[i][j] = gs.charData[i][j].clone(a)
		}
	}

	if gs.workingChar != nil {
		wc := gs.workingChar.clone(a)
		sys.workingChar = &wc
	} else {
		sys.workingChar = gs.workingChar
	}

	sys.charList = gs.charList.clone(a)
}

func (gs *GameState) loadSuperData(a *arena.Arena) {
	sys.super = gs.super
	sys.supertime = gs.supertime
	sys.superpausebg = gs.superpausebg
	sys.superendcmdbuftime = gs.superendcmdbuftime
	sys.superplayer = gs.superplayer
	sys.superdarken = gs.superdarken
	if gs.superanim != nil {
		sys.superanim = gs.superanim.clone(a)
	} else {
		sys.superanim = gs.superanim
	}
	sys.superpmap = gs.superpmap.clone(a)
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

func (gs *GameState) loadExplodData(a *arena.Arena) {
	for i := range gs.explods {
		sys.explods[i] = arena.MakeSlice[Explod](a, len(gs.explods[i]), len(gs.explods[i]))
		for j := 0; j < len(gs.explods[i]); j++ {
			sys.explods[i][j] = *gs.explods[i][j].clone(a)
		}
	}

	for i := range gs.explDrawlist {
		sys.explDrawlist[i] = arena.MakeSlice[int](a, len(gs.explDrawlist[i]), len(gs.explDrawlist[i]))
		copy(sys.explDrawlist[i], gs.explDrawlist[i])
	}

	for i := range gs.topexplDrawlist {
		sys.topexplDrawlist[i] = arena.MakeSlice[int](a, len(gs.topexplDrawlist[i]), len(gs.topexplDrawlist[i]))
		copy(sys.topexplDrawlist[i], gs.topexplDrawlist[i])
	}

	for i := range gs.underexplDrawlist {
		sys.underexplDrawlist[i] = arena.MakeSlice[int](a, len(gs.underexplDrawlist[i]), len(gs.underexplDrawlist[i]))
		copy(sys.underexplDrawlist[i], gs.underexplDrawlist[i])
	}
}

func (gs *GameState) projectliesPersist() bool {
	for i := 0; i < len(sys.projs); i++ {
		if len(sys.projs[i]) != len(gs.projs[i]) {
			return false
		}
		for j := 0; j < len(sys.projs[i]); j++ {
			if sys.projs[i][j].id != gs.projs[i][j].id {
				return false
			}
		}
	}
	return true
}

func (gs *GameState) loadProjectileData(a *arena.Arena) {
	if gs.projectliesPersist() {
		for i := range sys.projs {
			for j := range sys.projs[i] {
				sys.projs[i][j] = gs.projs[i][j].clone(a)
			}
		}
	} else {
		for i := range gs.projs {
			sys.projs[i] = arena.MakeSlice[Projectile](a, len(gs.projs[i]), len(gs.projs[i]))
			for j := range gs.projs[i] {
				sys.projs[i][j] = gs.projs[i][j].clone(a)
			}
		}

	}

}

func PoolAlloc(item interface{}) (result interface{}) {
	switch item.(type) {
	case (map[int32][3]*HitScale):
		return sys.statePool.hitscaleMapPool.Get()
	case (map[string]float32):
		return sys.statePool.stringFloat32MapPool.Get()
	case (map[string]int):
		return sys.statePool.stringIntMapPool.Get()
	case (AnimationTable):
		return sys.statePool.animationTablePool.Get()
	case ([]map[string]float32):
		return sys.statePool.mapArraySlicePool.Get()
	case (map[int32]*Char):
		return sys.statePool.int32CharPointerMapPool.Get()
	default:
		return nil
	}
}

func NewGameStatePool() GameStatePool {
	return GameStatePool{
		gameStatePool: sync.Pool{
			New: func() interface{} {
				return NewGameState()
			},
		},
		stringIntMapPool: sync.Pool{
			New: func() interface{} {
				si := make(map[string]int)
				return &si
			},
		},
		hitscaleMapPool: sync.Pool{
			New: func() interface{} {
				hs := make(map[int32][3]*HitScale)
				return &hs
			},
		},
		stringFloat32MapPool: sync.Pool{
			New: func() interface{} {
				sf := make(map[string]float32)
				return &sf
			},
		},
		animationTablePool: sync.Pool{
			New: func() interface{} {
				at := make(AnimationTable)
				return &at
			},
		},
		mapArraySlicePool: sync.Pool{
			New: func() interface{} {
				ma := make([]map[string]float32, 0, 8)
				return &ma
			},
		},
		int32CharPointerMapPool: sync.Pool{
			New: func() interface{} {
				ic := make(map[int32]*Char)
				return &ic
			},
		},
	}
}

func PreAllocHitScale() [3]*HitScale {
	h := [3]*HitScale{}
	for i := 0; i < len(h); i++ {
		h[i] = &HitScale{}
	}
	return h
}

type GameStatePool struct {
	gameStatePool           sync.Pool
	stringIntMapPool        sync.Pool
	hitscaleMapPool         sync.Pool
	stringFloat32MapPool    sync.Pool
	animationTablePool      sync.Pool
	mapArraySlicePool       sync.Pool
	int32CharPointerMapPool sync.Pool
}
