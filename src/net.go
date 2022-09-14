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
	sys.gameState.SaveState()
	g.saveStates[stateID] = &sys.gameState
	checksum := ggpo.DefaultChecksum
	return checksum
}

func (g *RollbackSession) LoadGameState(stateID int) {
	g.saveStates[stateID].LoadState()
}

func (g *RollbackSession) AdvanceFrame(flags int) {
	var discconectFlags int

	// Make sure we fetch the inputs from GGPO and use these to update
	// the game state instead of reading from the keyboard.
	inputs, result := g.backend.SyncInput(&discconectFlags)
	if result == nil {
		input := decodeInputs(inputs)
		sys.action(input)
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
	ggpo.DisableLogger()

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
	ggpo.DisableLogger()

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
