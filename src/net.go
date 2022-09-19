package main

import (
	"fmt"
	"os"
	"time"

	ggpo "github.com/assemblaj/GGPO-Go/pkg"
)

type RollbackSession struct {
	backend      ggpo.Backend
	saveStates   map[int]*GameState
	now          int64
	next         int64
	players      []ggpo.Player
	handles      []ggpo.PlayerHandle
	rep          *os.File
	connected    bool
	host         string
	playerNo     int
	syncProgress int
	synchronized bool
	syncTest     bool
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
	gs := NewGameState()
	g.saveStates[stateID] = gs
	g.saveStates[stateID].SaveState()

	fmt.Printf("Save state for stateID: %d\n", stateID)
	fmt.Println(g.saveStates[stateID])

	checksum := g.saveStates[stateID].Checksum()
	fmt.Printf("checksum: %d\n", checksum)
	return checksum
}

func (g *RollbackSession) LoadGameState(stateID int) {
	fmt.Printf("Loaded state for stateID: %d\n", stateID)
	fmt.Println(g.saveStates[stateID])

	g.saveStates[stateID].LoadState()
	//sys.gameStatePool <- g.saveStates[stateID]
}

func (g *RollbackSession) AdvanceFrame(flags int) {
	var discconectFlags int

	// Make sure we fetch the inputs from GGPO and use these to update
	// the game state instead of reading from the keyboard.
	inputs, result := g.backend.SyncInput(&discconectFlags)
	if result == nil {
		fmt.Println("Advancing frame from within callback.")
		if sys.roundState() == 0 || sys.roundState() == 1 {
			sys.currentFight.initChars()
		}
		//var finish bool
		input := decodeInputs(inputs)
		fmt.Println(input)
		sys.step = false
		sys.runShortcutScripts()
		// If next round
		sys.runNextRound()
		sys.updateStage()
		sys.action(input)
		sys.handleFlags()
		sys.updateEvents()

		// Break if finished
		//finish = sys.currentFight.fin && (!sys.postMatchFlg || len(sys.commonLua) == 0)
		//fmt.Println(finish)
		// Update system; break if update returns false (game ended)
		//if !s.update() {
		//	return false
		//}

		// If end match selected from menu/end of attract mode match/etc
		if sys.endMatch {
			sys.esc = true
		} else if sys.esc {
			sys.endMatch = sys.netInput != nil || len(sys.commonLua) == 0
		}

		sys.updateCamera()

		err := g.backend.AdvanceFrame()
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
	case ggpo.EventCodeTimeSync:
		fmt.Println("I'm sleeping\n\n\n\n")
		time.Sleep(time.Millisecond * time.Duration(info.FramesAhead/60))
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
	return r

}
func encodeInputs(inputs InputBits) []byte {
	return writeI32(int32(inputs))
}

func GameInitP1(numPlayers int, localPort int, remoteIp int, host string) {
	ggpo.EnableLogger()

	var inputBits InputBits = 0
	var inputSize int = len(encodeInputs(inputBits))

	player := ggpo.NewLocalPlayer(20, 1)
	player2 := ggpo.NewRemotePlayer(20, 2, host, remoteIp)
	sys.rollbackNetwork.players = append(sys.rollbackNetwork.players, player)
	sys.rollbackNetwork.players = append(sys.rollbackNetwork.players, player2)

	peer := ggpo.NewPeer(sys.rollbackNetwork, localPort, numPlayers, inputSize)
	//peer := ggpo.NewSyncTest(sys.rollbackNetwork, numPlayers, 8, inputSize)
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

func GameInitP2(numPlayers int, localPort int, remoteIp int, host string) {
	ggpo.EnableLogger()

	var inputBits InputBits = 0
	var inputSize int = len(encodeInputs(inputBits))

	player := ggpo.NewRemotePlayer(20, 1, host, remoteIp)
	player2 := ggpo.NewLocalPlayer(20, 2)
	sys.rollbackNetwork.players = append(sys.rollbackNetwork.players, player)
	sys.rollbackNetwork.players = append(sys.rollbackNetwork.players, player2)

	peer := ggpo.NewPeer(sys.rollbackNetwork, localPort, numPlayers, inputSize)
	//peer := ggpo.NewSyncTest(sys.rollbackNetwork, numPlayers, 8, inputSize)
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
	peer := ggpo.NewSyncTest(sys.rollbackNetwork, numPlayers, 8, inputSize)
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
