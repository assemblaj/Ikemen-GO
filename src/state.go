package main

type GameState struct {
	randseed           int32
	time               int32
	gameTime           int32
	chars              [MaxSimul*2 + MaxAttachedChar][]*Char
	charList           CharList
	explods            [MaxSimul*2 + MaxAttachedChar][]Explod
	explDrawlist       [MaxSimul*2 + MaxAttachedChar][]int
	topexplDrawlist    [MaxSimul*2 + MaxAttachedChar][]int
	underexplDrawlist  [MaxSimul*2 + MaxAttachedChar][]int
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
	superpmap          PalFX
	superpos           [2]float32
	superfacing        float32
	superp2defmul      float32
}

func NewGameState() GameState {
	return GameState{}
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
}

func (gs *GameState) savePalFX() {
	gs.allPalFX = sys.allPalFX
	gs.bgPalFX = sys.bgPalFX
}

func (gs *GameState) saveCharData() {
	for i := range sys.chars {
		for j, c := range sys.chars[i] {
			gs.chars[i][j] = c.clone()
		}
	}
	gs.charList = *sys.charList.clone()
}

func (gs *GameState) saveSuperData() {
	gs.super = sys.super
	gs.supertime = sys.supertime
	gs.superpausebg = sys.superpausebg
	gs.superendcmdbuftime = sys.superendcmdbuftime
	gs.superplayer = sys.superplayer
	gs.superdarken = sys.superdarken
	gs.superanim = sys.superanim
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
		gs.explods[i] = make([]Explod, len(gs.explods[i]))
		copy(gs.explods[i], sys.explods[i])
	}

	for i := range sys.explDrawlist {
		gs.explDrawlist[i] = make([]int, len(gs.explDrawlist[i]))
		copy(gs.explDrawlist[i], sys.explDrawlist[i])
	}

	for i := range sys.topexplDrawlist {
		gs.topexplDrawlist[i] = make([]int, len(gs.topexplDrawlist[i]))
		copy(gs.topexplDrawlist[i], sys.topexplDrawlist[i])
	}

	for i := range sys.underexplDrawlist {
		gs.underexplDrawlist[i] = make([]int, len(gs.underexplDrawlist[i]))
		copy(gs.underexplDrawlist[i], sys.underexplDrawlist[i])
	}
}

func (gs *GameState) clone() (result *GameState) {
	result = &GameState{}
	*result = *gs

	// Manually copy references that shallow copy poorly, as needed
	// Pointers, slices, maps, functions, channels etc
	result.chars = [9][]*Char{}
	for i := range gs.chars {
		for j, c := range gs.chars[i] {
			result.chars[i][j] = c.clone()
		}
	}

	for i := range gs.chars {
		result.chars[i] = make([]*Char, len(gs.chars[i]))
		copy(result.chars[i], gs.chars[i])
	}

	result.charList = *gs.charList.clone()

	for i := range gs.explods {
		result.explods[i] = make([]Explod, len(gs.explods[i]))
		copy(result.explods[i], gs.explods[i])
	}

	for i := range gs.explDrawlist {
		result.explDrawlist[i] = make([]int, len(gs.explDrawlist[i]))
		copy(result.explDrawlist[i], gs.explDrawlist[i])
	}

	for i := range gs.topexplDrawlist {
		result.topexplDrawlist[i] = make([]int, len(gs.topexplDrawlist[i]))
		copy(result.topexplDrawlist[i], gs.topexplDrawlist[i])
	}

	for i := range gs.underexplDrawlist {
		result.underexplDrawlist[i] = make([]int, len(gs.underexplDrawlist[i]))
		copy(result.underexplDrawlist[i], gs.underexplDrawlist[i])
	}

	return
}
func (gs *GameState) loadPalFX() {
	sys.allPalFX = gs.allPalFX
	sys.bgPalFX = gs.bgPalFX
}

func (gs *GameState) loadCharData() {
	for i := range gs.chars {
		for j, c := range sys.chars[i] {
			sys.chars[i][j] = c.clone()
		}
	}
	sys.charList = *gs.charList.clone()
}

func (gs *GameState) loadSuperData() {
	sys.super = gs.super
	sys.supertime = gs.supertime
	sys.superpausebg = gs.superpausebg
	sys.superendcmdbuftime = gs.superendcmdbuftime
	sys.superplayer = gs.superplayer
	sys.superdarken = gs.superdarken
	sys.superanim = gs.superanim
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
	for i := range gs.explods {
		sys.explods[i] = make([]Explod, len(gs.explods[i]))
		copy(sys.explods[i], gs.explods[i])
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
