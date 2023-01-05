package main

import (
	"fmt"
	"os"
	"time"

	ggpo "github.com/assemblaj/GGPO-Go/pkg"
)

type RollbackSession struct {
	backend             ggpo.Backend
	saveStates          map[int]*GameState
	now                 int64
	next                int64
	players             []ggpo.Player
	handles             []ggpo.PlayerHandle
	rep                 *os.File
	connected           bool
	host                string
	playerNo            int
	syncProgress        int
	synchronized        bool
	syncTest            bool
	run                 int
	remoteIp            string
	currentPlayer       int
	currentPlayerHandle ggpo.PlayerHandle
	loopTimer           LoopTimer
}
type LoopTimer struct {
	lastAdvantage      float32
	usPergameLoop      int
	usExtraToWait      int
	framesToSpreadWait int
	waitCount          time.Duration
	timeWait           time.Duration
	waitTotal          time.Duration
}

func NewLoopTimer(fps uint32, framesToSpread uint32) LoopTimer {
	return LoopTimer{
		framesToSpreadWait: int(framesToSpread),
		usPergameLoop:      1000000 / int(fps),
	}
}

func (lt *LoopTimer) OnGGPOTimeSyncEvent(framesAhead float32) {
	lt.waitTotal = time.Duration(float32(time.Second/60) * framesAhead)
	lt.lastAdvantage = float32(time.Second/60) * framesAhead
	lt.lastAdvantage = lt.lastAdvantage / 2
	lt.timeWait = time.Duration(lt.lastAdvantage) / time.Duration(lt.framesToSpreadWait)
	lt.waitCount = time.Duration(lt.framesToSpreadWait)
}

func (lt *LoopTimer) usToWaitThisLoop() int64 {
	if lt.waitCount > 0 {
		// lt.waitTotal -= lt.timeWait
		lt.waitCount--
		return int64(lt.timeWait)
	}
	return 0
}

func (r *RollbackSession) Close() {
	if r.backend != nil {
		r.backend.Close()
	}
}

func (r *RollbackSession) IsConnected() bool {
	return r.connected
}

func (g *RollbackSession) SaveGameState(stateID int) int {
	// gs := NewGameState()
	// if len(sys.gameStatePool) > 0 {
	// 	fmt.Println("UsedPool")
	// 	gs = <-sys.gameStatePool
	// } else {
	// 	fmt.Println("Didn't use pool")
	// 	gs = NewGameState()
	// }
	g.saveStates[stateID] = sys.statePool.gameStatePool.Get().(*GameState)
	g.saveStates[stateID].SaveState()

	// fmt.Printf("Save state for stateID: %d\n", stateID)
	// fmt.Println(g.saveStates[stateID])

	// checksum := g.saveStates[stateID].Checksum()
	// fmt.Printf("checksum: %d\n", checksum)
	// return checksum
	return ggpo.DefaultChecksum
}

func (g *RollbackSession) LoadGameState(stateID int) {
	// fmt.Printf("Loaded state for stateID: %d\n", stateID)
	// fmt.Println(g.saveStates[stateID])

	// checksum := g.saveStates[stateID].Checksum()
	// fmt.Printf("checksum: %d\n", checksum)

	g.saveStates[stateID].LoadState()
	sys.statePool.gameStatePool.Put(g.saveStates[stateID])
	//sys.gameStatePool <- g.saveStates[stateID]
}

func (g *RollbackSession) AdvanceFrame(flags int) {
	var discconectFlags int
	//Lua code executed before drawing fade, clsns and debug
	for _, str := range sys.commonLua {
		if err := sys.luaLState.DoString(str); err != nil {
			sys.luaLState.RaiseError(err.Error())
		}
	}

	// Make sure we fetch the inputs from GGPO and use these to update
	// the game state instead of reading from the keyboard.
	inputs, result := g.backend.SyncInput(&discconectFlags)
	if result == nil {
		//fmt.Println("Advancing frame from within callback.")
		input := decodeInputs(inputs)
		//fmt.Printf("Inputs: %v\n", input)

		sys.step = false
		sys.runShortcutScripts()
		// If next round
		sys.runNextRound()

		sys.updateStage()
		sys.action(input)

		sys.handleFlags()
		sys.updateEvents()

		if sys.endMatch {
			sys.esc = true
		} else if sys.esc {
			sys.endMatch = sys.netInput != nil || len(sys.commonLua) == 0
		}

		sys.updateCamera()
		err := g.backend.AdvanceFrame(ggpo.DefaultChecksum)
		if err != nil {
			panic(err)
		}
	}
}

func (g *RollbackSession) OnEvent(info *ggpo.Event) {
	switch info.Code {
	case ggpo.EventCodeConnectedToPeer:
		g.connected = true
	case ggpo.EventCodeSynchronizingWithPeer:
		g.syncProgress = 100 * (info.Count / info.Total)
	case ggpo.EventCodeSynchronizedWithPeer:
		g.syncProgress = 100
		g.synchronized = true
	case ggpo.EventCodeRunning:
		fmt.Println("EventCodeRunning")
	case ggpo.EventCodeDisconnectedFromPeer:
		fmt.Println("EventCodeDisconnectedFromPeer")
		sys.currentFight.fin = true
		sys.endMatch = true
		sys.esc = true
		disconnectMessage := fmt.Sprintf("Player %d disconnected.", info.Player)
		ShowInfoDialog(disconnectMessage, "Disconenection")
	case ggpo.EventCodeTimeSync:
		fmt.Printf("EventCodeTimeSync: FramesAhead %f TimeSyncPeriodInFrames: %d\n", info.FramesAhead, info.TimeSyncPeriodInFrames)
		g.loopTimer.OnGGPOTimeSyncEvent(info.FramesAhead)
	case ggpo.EventCodeConnectionInterrupted:
		fmt.Println("EventCodeconnectionInterrupted")
	case ggpo.EventCodeConnectionResumed:
		fmt.Println("EventCodeconnectionInterrupted")
	}
}

func NewRollbackSesesion() RollbackSession {
	r := RollbackSession{}
	r.saveStates = make(map[int]*GameState)
	r.players = make([]ggpo.Player, 9)
	r.handles = make([]ggpo.PlayerHandle, 9)
	r.loopTimer = NewLoopTimer(60, 200)
	return r
}
func encodeInputs(inputs InputBits) []byte {
	return writeI32(int32(inputs))
}

func GameInitP1(numPlayers int, localPort int, remotePort int, remoteIp string) {
	//ggpo.EnableLogger()
	ggpo.EnableLogger()
	logFileName := "Player1.log"
	f, err := os.OpenFile(logFileName, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	ggpo.SetLoggerOutput(f)

	var inputBits InputBits = 0
	var inputSize int = len(encodeInputs(inputBits))

	player := ggpo.NewLocalPlayer(20, 1)
	player2 := ggpo.NewRemotePlayer(20, 2, remoteIp, remotePort)
	sys.rollbackNetwork.players = append(sys.rollbackNetwork.players, player)
	sys.rollbackNetwork.players = append(sys.rollbackNetwork.players, player2)

	peer := ggpo.NewPeer(sys.rollbackNetwork, localPort, numPlayers, inputSize)
	sys.rollbackNetwork.backend = &peer

	peer.InitializeConnection()
	peer.Start()

	var handle ggpo.PlayerHandle
	result := peer.AddPlayer(&player, &handle)
	if result != nil {
		panic("panic")
	}
	sys.rollbackNetwork.handles = append(sys.rollbackNetwork.handles, handle)
	var handle2 ggpo.PlayerHandle
	result = peer.AddPlayer(&player2, &handle2)
	if result != nil {
		panic("panic")
	}
	sys.rollbackNetwork.handles = append(sys.rollbackNetwork.handles, handle2)
	sys.rollbackNetwork.currentPlayer = int(handle)
	sys.rollbackNetwork.currentPlayerHandle = handle

	peer.SetDisconnectTimeout(3000)
	peer.SetDisconnectNotifyStart(1000)
}

func GameInitP2(numPlayers int, localPort int, remotePort int, remoteIp string) {
	ggpo.EnableLogger()
	logFileName := "Player1.log"
	f, err := os.OpenFile(logFileName, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	ggpo.SetLoggerOutput(f)

	var inputBits InputBits = 0
	var inputSize int = len(encodeInputs(inputBits))

	player := ggpo.NewRemotePlayer(20, 1, remoteIp, remotePort)
	player2 := ggpo.NewLocalPlayer(20, 2)
	sys.rollbackNetwork.players = append(sys.rollbackNetwork.players, player)
	sys.rollbackNetwork.players = append(sys.rollbackNetwork.players, player2)

	peer := ggpo.NewPeer(sys.rollbackNetwork, localPort, numPlayers, inputSize)
	sys.rollbackNetwork.backend = &peer

	peer.InitializeConnection()
	peer.Start()

	var handle ggpo.PlayerHandle
	result := peer.AddPlayer(&player, &handle)
	if result != nil {
		panic("panic")
	}
	sys.rollbackNetwork.handles = append(sys.rollbackNetwork.handles, handle)
	var handle2 ggpo.PlayerHandle
	result = peer.AddPlayer(&player2, &handle2)
	if result != nil {
		panic("panic")
	}
	sys.rollbackNetwork.handles = append(sys.rollbackNetwork.handles, handle2)
	sys.rollbackNetwork.currentPlayer = int(handle2)
	sys.rollbackNetwork.currentPlayerHandle = handle2

	peer.SetDisconnectTimeout(3000)
	peer.SetDisconnectNotifyStart(1000)
}

func GameInitSyncTest(numPlayers int) {
	rs := NewRollbackSesesion()
	sys.rollbackNetwork = &rs
	sys.rollbackNetwork.syncTest = true

	ggpo.EnableLogger()

	var inputBits InputBits = 0
	var inputSize int = len(encodeInputs(inputBits))

	player := ggpo.NewLocalPlayer(20, 1)
	player2 := ggpo.NewLocalPlayer(20, 2)
	sys.rollbackNetwork.players = append(sys.rollbackNetwork.players, player)
	sys.rollbackNetwork.players = append(sys.rollbackNetwork.players, player2)

	//peer := ggpo.NewPeer(sys.rollbackNetwork, localPort, numPlayers, inputSize)
	peer := ggpo.NewSyncTest(sys.rollbackNetwork, numPlayers, 8, inputSize, true)
	sys.rollbackNetwork.backend = &peer

	//
	peer.InitializeConnection()
	peer.Start()

	var handle ggpo.PlayerHandle
	result := peer.AddPlayer(&player, &handle)
	if result != nil {
		panic("panic")
	}
	sys.rollbackNetwork.handles = append(sys.rollbackNetwork.handles, handle)
	var handle2 ggpo.PlayerHandle
	result = peer.AddPlayer(&player2, &handle2)
	if result != nil {
		panic("panic")
	}
	sys.rollbackNetwork.handles = append(sys.rollbackNetwork.handles, handle2)

	peer.SetDisconnectTimeout(3000)
	peer.SetDisconnectNotifyStart(1000)
}
