package main

import "fmt"

func init() {
	sys.gameState.randseed = sys.randseed
}

type CharState struct {
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
}

type AnimationState struct {
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
	// projectileState   [MaxSimul*2 + MaxAttachedChar][]ProjectileState
	// charState         [MaxSimul*2 + MaxAttachedChar][]CharState
	// explodsState      [MaxSimul*2 + MaxAttachedChar][]ExplodState
	// explDrawlist      [MaxSimul*2 + MaxAttachedChar][]int
	// topexplDrawlist   [MaxSimul*2 + MaxAttachedChar][]int
	// underexplDrawlist [MaxSimul*2 + MaxAttachedChar][]int
	// aiInput           [MaxSimul*2 + MaxAttachedChar]AiInput

	// charMap map[int32]CharState
	// projMap map[int32]ProjectileState

	// com                [MaxSimul*2 + MaxAttachedChar]float32
	// cam                Camera
	// allPalFX           PalFX
	// bgPalFX            PalFX
	// pause              int32
	// pausetime          int32
	// pausebg            bool
	// pauseendcmdbuftime int32
	// pauseplayer        int
	// super              int32
	// supertime          int32
	// superpausebg       bool
	// superendcmdbuftime int32
	// superplayer        int
	// superdarken        bool
	// superanim          AnimationState
	// superanimRef       *Animation
	// superpmap          PalFX
	// superpos           [2]float32
	// superfacing        float32
	// superp2defmul      float32

	// envShake            EnvShake
	// specialFlag         GlobalSpecialFlag
	// envcol              [3]int32
	// envcol_time         int32
	// bcStack, bcVarStack BytecodeStack
	// bcVar               []BytecodeValue
	// stageState          StageState

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
		for i := range sys.projs {
			fmt.Printf("len of projs %d len of projsState %d\n", len(sys.projs[i]), len(gs.projectileState[i]))
			if len(sys.projs[i]) < len(gs.projectileState[i]) {
				for len(sys.projs[i]) < len(gs.projectileState[i]) {
					sys.chars[i][0].newProj()
				}
			} else if len(sys.projs[i]) > len(gs.projectileState[i]) {
				for len(sys.projs[i]) > len(gs.projectileState[i]) {
					sys.projs[i] = sys.projs[i][:len(sys.projs[i])-1]
				}
			}
		}
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

		for i := range sys.projs {
			for j := range sys.projs[i] {
				sys.projs[i][j].loadProjectileState(gs.projectileState[i][j])
			}
		}

	}

}
