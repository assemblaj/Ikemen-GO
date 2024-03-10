package main

import (
	"math"
	"strings"
	"time"
	
	ggpo "github.com/assemblaj/ggpo"
	lua "github.com/yuin/gopher-lua"
)

type RollbackSystem struct {
	session      *RollbackSession
	currentFight Fight
	active       bool
	netInput     *NetInput
}

type RollbackConfig struct {
	FrameDelay            int  `json:"frameDelay"`
	DisconnectNotifyStart int  `json:"disconnectNotifyStart"`
	DisconnectTimeout     int  `json:"disconnectTimeout"`
	LogsEnabled           bool `json:"logsEnabled"`
	DesyncTest            bool `json:"desyncTest"`
	DesyncTestFrames      int  `json:"desyncTestFrames"`
	DesyncTestAI          bool `jaon:"desyncTestAI"`
}

func (rs *RollbackSystem) fight(s *System) bool {
	// Reset variables
	s.gameTime, s.paused, s.accel = 0, false, 1
	s.aiInput = [len(s.aiInput)]AiInput{}
	rs.currentFight = NewFight()
	// Defer resetting variables on return
	defer rs.currentFight.endFight()

	s.debugWC = sys.chars[0][0]
	// debugInput := func() {
	// 	select {
	// 	case cl := <-s.commandLine:
	// 		if err := s.luaLState.DoString(cl); err != nil {
	// 			s.errLog.Println(err.Error())
	// 		}
	// 	default:
	// 	}
	// }

	// Synchronize with external inputs (netplay, replays, etc)
	if err := s.synchronize(); err != nil {
		s.errLog.Println(err.Error())
		s.esc = true
	}
	if s.netInput != nil {
		defer s.netInput.Stop()
	}
	s.wincnt.init()

	// Initialize super meter values, and max power for teams sharing meter
	rs.currentFight.initSuperMeter()
	rs.currentFight.initTeamsLevels()

	rs.currentFight.initChars()

	//default bgm playback, used only in Quick VS or if externalized Lua implementaion is disabled
	if s.round == 1 && (s.gameMode == "" || len(sys.commonLua) == 0) {
		s.bgm.Open(s.stage.bgmusic, 1, int(s.stage.bgmvolume), int(s.stage.bgmloopstart), int(s.stage.bgmloopend), 0)
	}

	rs.currentFight.oldWins, rs.currentFight.oldDraws = s.wins, s.draws
	rs.currentFight.oldTeamLeader = s.teamLeader

	var running bool
	if rs.session != nil && s.netInput != nil {
		if rs.session.host != "" {
			rs.session.InitP2(2, 7550, 7600, rs.session.host)
			rs.session.playerNo = 2
		} else {
			rs.session.InitP1(2, 7600, 7550, rs.session.remoteIp)
			rs.session.playerNo = 1
		}
		s.time = rs.session.netTime
		s.preFightTime = s.netInput.preFightTime
		//if !rs.session.IsConnected() {
		// for !rs.session.synchronized {
		// 	rs.session.backend.Idle(0)
		// }
		//}
		// s.netInput.Close()
		rs.session.rep = s.netInput.rep
		rs.netInput = s.netInput
		s.netInput = nil
	} else if s.netInput == nil && rs.session == nil {
		session := NewRollbackSesesion(s.rollbackConfig)
		rs.session = &session
		rs.session.InitSyncTest(2)
	}
	rs.session.netTime = 0
	rs.currentFight.reset()
	// Loop until end of match
	///fin := false
	for !s.endMatch {
		rs.session.now = time.Now().UnixMilli()
		err := rs.session.backend.Idle(
			int(math.Max(0, float64(rs.session.next-rs.session.now-1))))
		if err != nil {
			panic(err)
		}

		running = rs.runFrame(s)

		if rs.currentFight.fin && (!s.postMatchFlg || len(s.commonLua) == 0) {
			break
		}

		rs.session.next = rs.session.now + 1000/60

		if !running {
			break
		}
		rs.render(s)
		frameTime := rs.session.loopTimer.usToWaitThisLoop()
		running = rs.update(s, frameTime)

		if !running {
			break
		}
	}
	rs.session.SaveReplay()
	// sys.esc = true
	//sys.rollback.currentFight.fin = true
	s.netInput = rs.netInput
	rs.session.backend.Close()

	// Prep for the next match.
	if s.netInput != nil {
		newSession := NewRollbackSesesion(sys.rollbackConfig)
		host := rs.session.host
		remoteIp := rs.session.remoteIp

		rs.session = &newSession
		rs.session.host = host
		rs.session.remoteIp = remoteIp
	} else {
		rs.session = nil
	}
	return false
}

func (rs *RollbackSystem) runFrame(s *System) bool {
	var buffer []byte
	var result error
	if rs.session.syncTest && rs.session.netTime == 0 {
		if !rs.session.config.DesyncTestAI {
			buffer = getInputs(0)
			result = rs.session.backend.AddLocalInput(ggpo.PlayerHandle(0), buffer, len(buffer))
			buffer = getInputs(1)
			result = rs.session.backend.AddLocalInput(ggpo.PlayerHandle(1), buffer, len(buffer))
		} else {
			buffer = getAIInputs(0)
			result = rs.session.backend.AddLocalInput(ggpo.PlayerHandle(0), buffer, len(buffer))
			buffer = getAIInputs(1)
			result = rs.session.backend.AddLocalInput(ggpo.PlayerHandle(1), buffer, len(buffer))
		}
	} else {
		buffer = getInputs(0)
		result = rs.session.backend.AddLocalInput(rs.session.currentPlayerHandle, buffer, len(buffer))
	}

	if result == nil {
		var values [][]byte
		disconnectFlags := 0
		values, result = rs.session.backend.SyncInput(&disconnectFlags)
		inputs := decodeInputs(values)
		if rs.session.rep != nil {
			rs.session.SetInput(rs.session.netTime, 0, inputs[0])
			rs.session.SetInput(rs.session.netTime, 1, inputs[1])
			rs.session.netTime++
		}

		if result == nil {

			s.step = false
			//rs.runShortcutScripts(s)

			// If next round
			if !rs.runNextRound(s) {
				return false
			}

			s.bgPalFX.step()
			s.stage.action()

			// If frame is ready to tick and not paused
			//rs.updateStage(s)

			// update lua
			for i := 0; i < len(inputs) && i < len(sys.commandLists); i++ {
				sys.commandLists[i].Buffer.InputBits(inputs[i], 1)
				sys.commandLists[i].Step(1, false, false, 0)
			}

			// Update game state
			rs.action(s, inputs)

			// if rs.handleFlags(s) {
			// 	return true
			// }

			if !rs.updateEvents(s) {
				return false
			}

			// Break if finished
			if rs.currentFight.fin && (!s.postMatchFlg || len(s.commonLua) == 0) {
				return false
			}

			// Update system; break if update returns false (game ended)
			//if !s.update() {
			//	return false
			//}

			// If end match selected from menu/end of attract mode match/etc
			if s.endMatch {
				s.esc = true
				return false
			} else if s.esc {
				s.endMatch = s.netInput != nil || len(sys.commonLua) == 0
				return false
			}

			defer func() {
				if re := recover(); re != nil {
					if rs.session.config.DesyncTest {
						rs.session.log.updateLogs()
						rs.session.log.saveLogs()
						panic("RaiseDesyncError")
					}
				}
			}()

			err := rs.session.backend.AdvanceFrame(rs.session.LiveChecksum(s))
			if err != nil {
				panic(err)
			}

		}
	}
	return true

}

func (rs *RollbackSystem) runShortcutScripts(s *System) {
	for _, v := range s.shortcutScripts {
		if v.Activate {
			if err := s.luaLState.DoString(v.Script); err != nil {
				s.errLog.Println(err.Error())
			}
		}
	}
}

func (rs *RollbackSystem) runNextRound(s *System) bool {
	if s.roundOver() && !rs.currentFight.fin {
		s.round++
		for i := range s.roundsExisted {
			s.roundsExisted[i]++
		}
		s.clearAllSound()
		tbl_roundNo := s.luaLState.NewTable()
		for _, p := range s.chars {
			if len(p) > 0 && p[0].teamside != -1 {
				tmp := s.luaLState.NewTable()
				tmp.RawSetString("name", lua.LString(p[0].name))
				tmp.RawSetString("id", lua.LNumber(p[0].id))
				tmp.RawSetString("memberNo", lua.LNumber(p[0].memberNo))
				tmp.RawSetString("selectNo", lua.LNumber(p[0].selectNo))
				tmp.RawSetString("teamside", lua.LNumber(p[0].teamside))
				tmp.RawSetString("life", lua.LNumber(p[0].life))
				tmp.RawSetString("lifeMax", lua.LNumber(p[0].lifeMax))
				tmp.RawSetString("winquote", lua.LNumber(p[0].winquote))
				tmp.RawSetString("aiLevel", lua.LNumber(p[0].aiLevel()))
				tmp.RawSetString("palno", lua.LNumber(p[0].palno()))
				tmp.RawSetString("ratiolevel", lua.LNumber(p[0].ocd().ratioLevel))
				tmp.RawSetString("win", lua.LBool(p[0].win()))
				tmp.RawSetString("winKO", lua.LBool(p[0].winKO()))
				tmp.RawSetString("winTime", lua.LBool(p[0].winTime()))
				tmp.RawSetString("winPerfect", lua.LBool(p[0].winPerfect()))
				tmp.RawSetString("winSpecial", lua.LBool(p[0].winType(WT_S)))
				tmp.RawSetString("winHyper", lua.LBool(p[0].winType(WT_H)))
				tmp.RawSetString("drawgame", lua.LBool(p[0].drawgame()))
				tmp.RawSetString("ko", lua.LBool(p[0].scf(SCF_ko)))
				tmp.RawSetString("ko_round_middle", lua.LBool(p[0].scf(SCF_ko_round_middle)))
				tbl_roundNo.RawSetInt(p[0].playerNo+1, tmp)
			}
		}
		s.matchData.RawSetInt(int(s.round-1), tbl_roundNo)
		s.scoreRounds = append(s.scoreRounds, [2]float32{s.lifebar.sc[0].scorePoints, s.lifebar.sc[1].scorePoints})
		rs.currentFight.oldTeamLeader = s.teamLeader

		if !s.matchOver() && (s.tmode[0] != TM_Turns || s.chars[0][0].win()) &&
			(s.tmode[1] != TM_Turns || s.chars[1][0].win()) {
			/* Prepare for the next round */
			for i, p := range s.chars {
				if len(p) > 0 {
					if s.tmode[i&1] != TM_Turns || !p[0].win() {
						p[0].life = p[0].lifeMax
					} else if p[0].life <= 0 {
						p[0].life = 1
					}
					p[0].redLife = 0
					rs.currentFight.copyVar(i)
				}
			}
			rs.currentFight.oldWins, rs.currentFight.oldDraws = s.wins, s.draws
			rs.currentFight.oldStageVars.copyStageVars(s.stage)
			rs.currentFight.reset()
		} else {
			/* End match, or prepare for a new character in turns mode */
			for i, tm := range s.tmode {
				if s.chars[i][0].win() || !s.chars[i][0].lose() && tm != TM_Turns {
					for j := i; j < len(s.chars); j += 2 {
						if len(s.chars[j]) > 0 {
							if s.chars[j][0].win() {
								s.chars[j][0].life = Max(1, int32(math.Ceil(math.Pow(rs.currentFight.lvmul,
									float64(rs.currentFight.level[i]))*float64(s.chars[j][0].life))))
							} else {
								s.chars[j][0].life = Max(1, s.cgi[j].data.life)
							}
						}
					}
					//} else {
					//	s.chars[i][0].life = 0
				}
			}
			// If match isn't over, presumably this is turns mode,
			// so break to restart fight for the next character
			if !s.matchOver() {
				return false
			}

			// Otherwise match is over
			s.postMatchFlg = true
			rs.currentFight.fin = true
		}
	}
	return true

}

func (rs *RollbackSystem) updateStage(s *System) {
	if s.tickFrame() && (s.super <= 0 || !s.superpausebg) &&
		(s.pause <= 0 || !s.pausebg) {
		// Update stage
		s.stage.action()
	}
}

func (rs *RollbackSystem) action(s *System, input []InputBits) {
	s.sprites = s.sprites[:0]
	s.topSprites = s.topSprites[:0]
	s.bottomSprites = s.bottomSprites[:0]
	s.shadows = s.shadows[:0]
	s.drawc1 = s.drawc1[:0]
	s.drawc2 = s.drawc2[:0]
	s.drawc2sp = s.drawc2sp[:0]
	s.drawc2mtk = s.drawc2mtk[:0]
	s.drawwh = s.drawwh[:0]
	s.clsnText = nil
	var x, y, scl float32 = s.cam.Pos[0], s.cam.Pos[1], s.cam.Scale / s.cam.BaseScale()
	var cvmin, cvmax, highest, lowest, leftest, rightest float32 = 0, 0, 0, 0, 0, 0
	leftest, rightest = x, x
	if s.cam.ytensionenable {
		if y < 0 {
			lowest = (y - s.cam.CameraZoomYBound)
		}
	}

	// Run lifebar
	if s.lifebar.ro.act() {
		if s.intro > s.lifebar.ro.ctrl_time {
			s.intro--
			if s.sf(GSF_intro) && s.intro <= s.lifebar.ro.ctrl_time {
				s.intro = s.lifebar.ro.ctrl_time + 1
			}
		} else if s.intro > 0 {
			if s.intro == s.lifebar.ro.ctrl_time {
				for _, p := range s.chars {
					if len(p) > 0 {
						if !p[0].sf(CSF_nointroreset) {
							p[0].posReset()
						}
					}
				}
			}
			s.intro--
			if s.intro == 0 {
				for _, p := range s.chars {
					if len(p) > 0 {
						p[0].unsetSCF(SCF_over)
						if !p[0].scf(SCF_standby) || p[0].teamside == -1 {
							p[0].setCtrl(true)
							if p[0].ss.no != 0 && !p[0].sf(CSF_nointroreset) {
								p[0].selfState(0, -1, -1, 1, "")
							}
						}
					}
				}
			}
		}
		if s.intro == 0 && s.time > 0 && !s.sf(GSF_timerfreeze) &&
			(s.super <= 0 || !s.superpausebg) && (s.pause <= 0 || !s.pausebg) {
			s.time--
		}
		fin := func() bool {
			if s.intro > 0 {
				return false
			}
			ko := [...]bool{true, true}
			for ii := range ko {
				for i := ii; i < MaxSimul*2; i += 2 {
					if len(s.chars[i]) > 0 && s.chars[i][0].teamside != -1 {
						if s.chars[i][0].alive() {
							ko[ii] = false
						} else if (s.tmode[i&1] == TM_Simul && s.loseSimul && s.com[i] == 0) ||
							(s.tmode[i&1] == TM_Tag && s.loseTag) {
							ko[ii] = true
							break
						}
					}
				}
				if ko[ii] {
					i := ii ^ 1
					for ; i < MaxSimul*2; i += 2 {
						if len(s.chars[i]) > 0 && s.chars[i][0].life <
							s.chars[i][0].lifeMax {
							break
						}
					}
					if i >= MaxSimul*2 {
						s.winType[ii^1].SetPerfect()
					}
				}
			}
			ft := s.finish
			if s.time == 0 {
				l := [2]float32{}
				for i := 0; i < 2; i++ {
					for j := i; j < MaxSimul*2; j += 2 {
						if len(s.chars[j]) > 0 {
							if s.tmode[i] == TM_Simul || s.tmode[i] == TM_Tag {
								l[i] += (float32(s.chars[j][0].life) /
									float32(s.numSimul[i])) /
									float32(s.chars[j][0].lifeMax)
							} else {
								l[i] += float32(s.chars[j][0].life) /
									float32(s.chars[j][0].lifeMax)
							}
						}
					}
				}
				if l[0] > l[1] {
					p := true
					for i := 0; i < MaxSimul*2; i += 2 {
						if len(s.chars[i]) > 0 &&
							s.chars[i][0].life < s.chars[i][0].lifeMax {
							p = false
							break
						}
					}
					if p {
						s.winType[0].SetPerfect()
					}
					s.finish = FT_TO
					s.winTeam = 0
				} else if l[0] < l[1] {
					p := true
					for i := 1; i < MaxSimul*2; i += 2 {
						if len(s.chars[i]) > 0 &&
							s.chars[i][0].life < s.chars[i][0].lifeMax {
							p = false
							break
						}
					}
					if p {
						s.winType[1].SetPerfect()
					}
					s.finish = FT_TO
					s.winTeam = 1
				} else {
					s.finish = FT_TODraw
					s.winTeam = -1
				}
				if !(ko[0] || ko[1]) {
					s.winType[0], s.winType[1] = WT_T, WT_T
				}
			}
			if s.intro >= -1 && (ko[0] || ko[1]) {
				if ko[0] && ko[1] {
					s.finish, s.winTeam = FT_DKO, -1
				} else {
					s.finish, s.winTeam = FT_KO, int(Btoi(ko[0]))
				}
			}
			if ft != s.finish {
				for i, p := range sys.chars {
					if len(p) > 0 && ko[^i&1] {
						for _, h := range p {
							for _, tid := range h.targets {
								if t := sys.playerID(tid); t != nil {
									if t.ghv.attr&int32(AT_AH) != 0 {
										s.winTrigger[i&1] = WT_H
									} else if t.ghv.attr&int32(AT_AS) != 0 &&
										s.winTrigger[i&1] == WT_N {
										s.winTrigger[i&1] = WT_S
									}
								}
							}
						}
					}
				}
			}
			return ko[0] || ko[1] || s.time == 0
		}
		if s.roundEnd() || fin() {
			inclWinCount := func() {
				w := [...]bool{!s.chars[1][0].win(), !s.chars[0][0].win()}
				if !w[0] || !w[1] ||
					s.tmode[0] == TM_Turns || s.tmode[1] == TM_Turns ||
					s.draws >= s.lifebar.ro.match_maxdrawgames[0] ||
					s.draws >= s.lifebar.ro.match_maxdrawgames[1] {
					for i, win := range w {
						if win {
							s.wins[i]++
							if s.matchOver() && s.wins[^i&1] == 0 {
								s.consecutiveWins[i]++
							}
							s.consecutiveWins[^i&1] = 0
						}
					}
				}
			}
			rs4t := -s.lifebar.ro.over_waittime
			s.intro--
			if s.intro == -s.lifebar.ro.over_hittime && s.finish != FT_NotYet {
				inclWinCount()
			}
			// Check if player skipped win pose time
			if s.roundWinTime() && (rs.session.AnyButtonIB(input) && !s.sf(GSF_roundnotskip)) {
				s.intro = Min(s.intro, rs4t-2-s.lifebar.ro.over_time+s.lifebar.ro.fadeout_time)
				s.winskipped = true
			}
			if s.winskipped || !s.roundWinTime() {
				// Check if game can proceed into roundstate 4
				if s.waitdown > 0 {
					if s.intro == rs4t-1 {
						for _, p := range s.chars {
							if len(p) > 0 {
								// Set inputwait flag to stop inputs until win pose time
								if !p[0].scf(SCF_inputwait) {
									p[0].setSCF(SCF_inputwait)
								}
								// Check if this character is ready to procced to roundstate 4
								if p[0].scf(SCF_over) || (p[0].scf(SCF_ctrl) && p[0].ss.moveType == MT_I &&
									p[0].ss.stateType != ST_A && p[0].ss.stateType != ST_L) {
									continue
								}
								// Freeze timer if any character is not ready to proceed yet
								s.intro = rs4t
								break
							}
						}
					}
				}
				// Disable ctrl (once) at the first frame of roundstate 4
				if s.intro == rs4t-1 {
					for _, p := range s.chars {
						if len(p) > 0 {
							p[0].setCtrl(false)
						}
					}
				}
				// Start running wintime counter only after getting into roundstate 4
				if s.intro < rs4t && !s.roundWinTime() {
					s.wintime--
				}
				// Set characters into win/lose poses, update win counters
				if s.waitdown <= 0 || s.roundWinTime() {
					if s.waitdown >= 0 {
						w := [...]bool{!s.chars[1][0].win(), !s.chars[0][0].win()}
						if !w[0] || !w[1] ||
							s.tmode[0] == TM_Turns || s.tmode[1] == TM_Turns ||
							s.draws >= s.lifebar.ro.match_maxdrawgames[0] ||
							s.draws >= s.lifebar.ro.match_maxdrawgames[1] {
							for i, win := range w {
								if win {
									s.lifebar.wi[i].add(s.winType[i])
									if s.matchOver() && s.wins[i] >= s.matchWins[i] {
										s.lifebar.wc[i].wins += 1
									}
								}
							}
						} else {
							s.draws++
						}
					}
					for _, p := range s.chars {
						if len(p) > 0 {
							//default life recovery, used only if externalized Lua implementaion is disabled
							if len(sys.commonLua) == 0 && s.waitdown >= 0 && s.time > 0 && p[0].win() &&
								p[0].alive() && !s.matchOver() &&
								(s.tmode[0] == TM_Turns || s.tmode[1] == TM_Turns) {
								p[0].life += int32((float32(p[0].lifeMax) *
									float32(s.time) / 60) * s.turnsRecoveryRate)
								if p[0].life > p[0].lifeMax {
									p[0].life = p[0].lifeMax
								}
							}
							if !p[0].scf(SCF_over) && !p[0].hitPause() && p[0].alive() && p[0].animNo != 5 {
								p[0].setSCF(SCF_over)
								p[0].unsetSCF(SCF_inputwait)
								if p[0].win() {
									p[0].selfState(180, -1, -1, -1, "")
								} else if p[0].lose() {
									p[0].selfState(170, -1, -1, -1, "")
								} else {
									p[0].selfState(175, -1, -1, -1, "")
								}
							}
						}
					}
					s.waitdown = 0
				}
				s.waitdown--
			}
			// If the game can't proceed to the fadeout screen, we turn back the counter 1 tick
			if !s.winskipped && s.sf(GSF_roundnotover) &&
				s.intro == rs4t-2-s.lifebar.ro.over_time+s.lifebar.ro.fadeout_time {
				s.intro++
			}
		} else if s.intro < 0 {
			s.intro = 0
		}
	}

	// Run tick frame
	if s.tickFrame() {
		s.xmin = s.cam.ScreenPos[0] + s.cam.Offset[0] + s.screenleft
		s.xmax = s.cam.ScreenPos[0] + s.cam.Offset[0] +
			float32(s.gameWidth)/s.cam.Scale - s.screenright
		if s.xmin > s.xmax {
			s.xmin = (s.xmin + s.xmax) / 2
			s.xmax = s.xmin
		}
		s.allPalFX.step()
		//s.bgPalFX.step()
		s.envShake.next()
		if s.envcol_time > 0 {
			s.envcol_time--
		}
		if s.enableZoomtime > 0 {
			s.enableZoomtime--
		} else {
			s.zoomCameraBound = true
			s.zoomStageBound = true
		}
		if s.super > 0 {
			s.super--
		} else if s.pause > 0 {
			s.pause--
		}
		if s.supertime < 0 {
			s.supertime = ^s.supertime
			s.super = s.supertime
		}
		if s.pausetime < 0 {
			s.pausetime = ^s.pausetime
			s.pause = s.pausetime
		}
		// in mugen 1.1 most global assertspecial flags are reset during pause
		// TODO: test if roundnotover should reset (keep intro and noko active)
		if s.super <= 0 && s.pause <= 0 {
			s.specialFlag = 0
		} else {
			s.unsetSF(GSF_assertspecialpause)
		}
		if s.superanim != nil {
			s.superanim.Action()
		}
		rs.rollbackAction(s, &s.charList, input, x, &cvmin, &cvmax,
			&highest, &lowest, &leftest, &rightest)
		s.nomusic = s.sf(GSF_nomusic) && !sys.postMatchFlg
	} else {
		s.charUpdate(&cvmin, &cvmax, &highest, &lowest, &leftest, &rightest)
	}
	s.lifebar.step()

	// Set global First Attack flag if either team got it
	if s.firstAttack[0] >= 0 || s.firstAttack[1] >= 0 {
		s.firstAttack[2] = 1
	}

	// Run camera
	leftest -= x
	rightest -= x
	var newx, newy float32 = x, y
	var sclMul float32
	sclMul = s.cam.action(&newx, &newy, leftest, rightest, lowest, highest,
		cvmin, cvmax, s.super > 0 || s.pause > 0)

	// Update camera
	introSkip := false
	if s.tickNextFrame() {
		if s.lifebar.ro.cur < 1 && !s.introSkipped {
			if s.shuttertime > 0 ||
				rs.session.AnyButtonIB(input) && !s.sf(GSF_roundnotskip) && s.intro > s.lifebar.ro.ctrl_time {
				s.shuttertime++
				if s.shuttertime == s.lifebar.ro.shutter_time {
					s.fadeintime = 0
					s.resetGblEffect()
					s.intro = s.lifebar.ro.ctrl_time
					for i, p := range s.chars {
						if len(p) > 0 {
							s.playerClear(i, false)
							p[0].posReset()
							p[0].selfState(0, -1, -1, 0, "")
						}
					}
					ox := newx
					newx = 0
					leftest = MaxF(float32(Min(s.stage.p[0].startx,
						s.stage.p[1].startx))*s.stage.localscl,
						-(float32(s.gameWidth)/2)/s.cam.BaseScale()+s.screenleft) - ox
					rightest = MinF(float32(Max(s.stage.p[0].startx,
						s.stage.p[1].startx))*s.stage.localscl,
						(float32(s.gameWidth)/2)/s.cam.BaseScale()-s.screenright) - ox
					introSkip = true
					s.introSkipped = true
				}
			}
		} else {
			if s.shuttertime > 0 {
				s.shuttertime--
			}
		}
	}
	if introSkip {
		sclMul = 1 / scl
	}
	leftest = (leftest - s.screenleft) * s.cam.BaseScale()
	rightest = (rightest + s.screenright) * s.cam.BaseScale()
	scl = s.cam.ScaleBound(scl, sclMul)
	tmp := (float32(s.gameWidth) / 2) / scl
	if AbsF((leftest+rightest)-(newx-x)*2) >= tmp/2 {
		tmp = MaxF(0, MinF(tmp, MaxF((newx-x)-leftest, rightest-(newx-x))))
	}
	x = s.cam.XBound(scl, MinF(x+leftest+tmp, MaxF(x+rightest-tmp, newx)))
	if !s.cam.ZoomEnable {
		// Pos X の誤差が出ないように精度を落とす
		x = float32(math.Ceil(float64(x)*4-0.5) / 4)
	}
	y = s.cam.YBound(scl, newy)
	s.cam.Update(scl, x, y)

	if s.superanim != nil {
		s.topSprites.add(&SprData{s.superanim, &s.superpmap, s.superpos,
			[...]float32{s.superfacing, 1}, [2]int32{-1}, 5, Rotation{}, [2]float32{},
			false, true, s.cgi[s.superplayer].ver[0] != 1, 1, 1, 0, 0, [4]float32{0, 0, 0, 0}}, 0, 0, 0, 0)
		if s.superanim.loopend {
			s.superanim = nil
		}
	}
	for i, pr := range s.projs {
		for j, p := range pr {
			if p.id >= 0 {
				s.projs[i][j].cueDraw(s.cgi[i].ver[0] != 1, i)
			}
		}
	}
	s.charList.cueDraw()
	explUpdate := func(edl *[len(s.chars)][]int, drop bool) {
		for i, el := range *edl {
			for j := len(el) - 1; j >= 0; j-- {
				if el[j] >= 0 {
					s.explods[i][el[j]].update(s.cgi[i].ver[0] != 1, i)
					if s.explods[i][el[j]].id == IErr {
						if drop {
							el = append(el[:j], el[j+1:]...)
							(*edl)[i] = el
						} else {
							el[j] = -1
						}
					}
				}
			}
		}
	}
	explUpdate(&s.explDrawlist, true)
	explUpdate(&s.topexplDrawlist, false)
	explUpdate(&s.underexplDrawlist, true)

	if s.tickNextFrame() {
		spd := s.gameSpeed * s.accel
		if s.postMatchFlg {
			spd = 1
		} else if !s.sf(GSF_nokoslow) && s.time != 0 && s.intro < 0 && s.slowtime > 0 {
			spd *= s.lifebar.ro.slow_speed
			if s.slowtime < s.lifebar.ro.slow_fadetime {
				spd += (float32(1) - s.lifebar.ro.slow_speed) * float32(s.lifebar.ro.slow_fadetime-s.slowtime) / float32(s.lifebar.ro.slow_fadetime)
			}
			s.slowtime--
		}
		s.turbo = spd
	}
	s.tickSound()
	return
}

func (rs *RollbackSystem) handleFlags(s *System) bool {
	// F4 pressed to restart round
	if s.roundResetFlg && !s.postMatchFlg {
		rs.currentFight.reset()
	}
	// Shift+F4 pressed to restart match
	if s.reloadFlg {
		return true
	}
	return false

}

func (rs *RollbackSystem) updateEvents(s *System) bool {
	if !s.addFrameTime(s.turbo) {
		if !s.eventUpdate() {
			return false
		}
		return false
	}
	return true
}

func (rs *RollbackSystem) updateCamera(s *System) {
	if !s.frameSkip {
		scl := s.cam.Scale / s.cam.BaseScale()
		if s.enableZoomtime > 0 {
			if !s.debugPaused() {
				s.zoomPosXLag += ((s.zoomPos[0] - s.zoomPosXLag) * (1 - s.zoomlag))
				s.zoomPosYLag += ((s.zoomPos[1] - s.zoomPosYLag) * (1 - s.zoomlag))
				s.drawScale = s.drawScale / (s.drawScale + (s.zoomScale*scl-s.drawScale)*s.zoomlag) * s.zoomScale * scl
			}
		} else {
			s.zoomlag = 0
			s.zoomPosXLag = 0
			s.zoomPosYLag = 0
			s.zoomScale = 1
			s.zoomPos = [2]float32{0, 0}
			s.drawScale = s.cam.Scale
		}
	}
}

func (rs *RollbackSystem) render(s *System) {
	if !s.frameSkip {
		x, y, scl := s.cam.Pos[0], s.cam.Pos[1], s.cam.Scale/s.cam.BaseScale()
		dx, dy, dscl := x, y, scl
		if s.enableZoomtime > 0 {
			if !s.debugPaused() {
				s.zoomPosXLag += ((s.zoomPos[0] - s.zoomPosXLag) * (1 - s.zoomlag))
				s.zoomPosYLag += ((s.zoomPos[1] - s.zoomPosYLag) * (1 - s.zoomlag))
				s.drawScale = s.drawScale / (s.drawScale + (s.zoomScale*scl-s.drawScale)*s.zoomlag) * s.zoomScale * scl
			}
			if s.zoomStageBound {
				dscl = MaxF(s.cam.MinScale, s.drawScale/s.cam.BaseScale())
				if s.zoomCameraBound {
					dx = x + ClampF(s.zoomPosXLag/scl, -s.cam.halfWidth/scl*2*(1-1/s.zoomScale), s.cam.halfWidth/scl*2*(1-1/s.zoomScale))
				} else {
					dx = x + s.zoomPosXLag/scl
				}
				dx = s.cam.XBound(dscl, dx)
			} else {
				dscl = s.drawScale / s.cam.BaseScale()
				dx = x + s.zoomPosXLag/scl
			}
			dy = y + s.zoomPosYLag/scl
		} else {
			s.zoomlag = 0
			s.zoomPosXLag = 0
			s.zoomPosYLag = 0
			s.zoomScale = 1
			s.zoomPos = [2]float32{0, 0}
			s.drawScale = s.cam.Scale
		}
		s.draw(dx, dy, dscl)
	}
	//Lua code executed before drawing fade, clsns and debug
	for _, str := range s.commonLua {
		if err := s.luaLState.DoString(str); err != nil {
			s.luaLState.RaiseError(err.Error())
		}
	}
	// Render debug elements
	if !s.frameSkip {
		s.drawTop()
		s.drawDebug()
	}
}

func (rs *RollbackSystem) update(s *System, wait time.Duration) bool {
	s.frameCounter++
	return rs.await(s, wait)
}

func (rs *RollbackSystem) await(s *System, wait time.Duration) bool {
	if !s.frameSkip {
		// Render the finished frame
		gfx.EndFrame()
		s.window.SwapBuffers()
		// Begin the next frame after events have been processed. Do not clear
		// the screen if network input is present.
		defer gfx.BeginFrame(sys.netInput == nil)
	}
	s.runMainThreadTask()
	now := time.Now()
	diff := s.redrawWait.nextTime.Sub(now)
	s.redrawWait.nextTime = s.redrawWait.nextTime.Add(wait)
	switch {
	case diff >= 0 && diff < wait+2*time.Millisecond:
		time.Sleep(diff)
		fallthrough
	case now.Sub(s.redrawWait.lastDraw) > 250*time.Millisecond:
		fallthrough
	case diff >= -17*time.Millisecond:
		s.redrawWait.lastDraw = now
		s.frameSkip = false
	default:
		if diff < -150*time.Millisecond {
			s.redrawWait.nextTime = now.Add(wait)
		}
		s.frameSkip = true
	}
	s.eventUpdate()
	return !s.gameEnd
}

func (rs *RollbackSystem) commandUpdate(ib []InputBits, sys *System) {
	for i, p := range sys.chars {
		if len(p) > 0 {
			r := p[0]
			act := true
			if sys.super > 0 {
				act = r.superMovetime != 0
			} else if sys.pause > 0 && r.pauseMovetime == 0 {
				act = false
			}
			// Having this here makes B and F inputs reverse the same instant the character turns
			if act && !r.sf(CSF_noautoturn) && (r.scf(SCF_ctrl) || r.roundState() > 2) &&
				(r.ss.no == 0 || r.ss.no == 11 || r.ss.no == 20 || r.ss.no == 52) {
				r.turn()
			}
			if r.inputOver() || r.sf(CSF_noinput) {
				for j := range r.cmd {
					r.cmd[j].BufReset()
				}
				continue
			}

			for _, c := range p {
				if c.helperIndex == 0 ||
					c.helperIndex > 0 && &c.cmd[0] != &r.cmd[0] {
					
					if c.key >= 0 {
						if c.playerNo >= 0 && c.playerNo < len(ib) && !strings.Contains(sys.gameMode, "coop") { 
							c.cmd[0].Buffer.InputBits(ib[c.playerNo], int32(c.facing))
						} else if strings.Contains(sys.gameMode, "coop") {
							var coopIdx int 
							if i > 1 {
								coopIdx = i - 1
							}
							c.cmd[0].Buffer.InputBits(ib[coopIdx],  int32(c.facing))
						}
					}
					/*
					else if i < len(ib) {
						if sys.gameMode == "watch" && (c.key < 0 && ^c.key < len(sys.aiInput)) {
							sys.aiInput[^c.key].Update(sys.com[i])
						}
						// if we have an input from the players
						// update the command buffer based on that.
						c.cmd[0].Buffer.InputBits(ib[i], int32(c.facing))
					} else if (sys.tmode[0] == TM_Tag || sys.tmode[1] == TM_Tag) && (r.teamside != -1) {
						c.cmd[0].Buffer.InputBits(ib[r.teamside], int32(c.facing))
					} else {
						// Otherwise, this will ostensibly update the buffers based on AIInput
						c.cmd[0].Input(c.key, int32(c.facing), sys.com[i], c.inputFlag)
					}*/
					hp := c.hitPause() && c.gi().constants["input.pauseonhitpause"] != 0
					buftime := Btoi(hp && c.gi().ver[0] != 1)
					if sys.super > 0 {
						if !act && sys.super <= sys.superendcmdbuftime {
							hp = true
						}
					} else if sys.pause > 0 {
						if !act && sys.pause <= sys.pauseendcmdbuftime {
							hp = true
						}
					}
					for j := range c.cmd {
						c.cmd[j].Step(int32(c.facing), c.key < 0, hp, buftime+Btoi(hp))
					}
				}
			}
			if r.key < 0 {
				cc := int32(-1)
				// AI Scaling
				// TODO: Balance AI Scaling
				if r.roundState() == 2 && RandF32(0, sys.com[i]/2+32) > 32 {
					cc = Rand(0, int32(len(r.cmd[r.ss.sb.playerNo].Commands))-1)
				} else {
					cc = -1
				}
				for j := range p {
					if p[j].helperIndex >= 0 {
						p[j].cpucmd = cc
					}
				}
			}
		}
	}
}

func (rs *RollbackSystem) rollbackAction(sys *System, cl *CharList, ib []InputBits,
	x float32, cvmin, cvmax, highest, lowest, leftest, rightest *float32) {
	rs.commandUpdate(ib, sys)
	// Prepare characters before performing their actions
	for i := 0; i < len(cl.runOrder); i++ {
		cl.runOrder[i].actionPrepare()
	}
	// Run character state controllers
	// Process priority based on movetype and player type
	// Run actions for attacking players and helpers
	for i := 0; i < len(cl.runOrder); i++ {
		if cl.runOrder[i].ss.moveType == MT_A {
			cl.runOrder[i].actionRun()
		}
	}
	// Run actions for idle players
	for i := 0; i < len(cl.runOrder); i++ {
		if cl.runOrder[i].helperIndex == 0 && cl.runOrder[i].ss.moveType == MT_I {
			cl.runOrder[i].actionRun()
		}
	}
	// Run actions for remaining players
	for i := 0; i < len(cl.runOrder); i++ {
		if cl.runOrder[i].helperIndex == 0 {
			cl.runOrder[i].actionRun()
		}
	}
	// Run actions for idle helpers
	for i := 0; i < len(cl.runOrder); i++ {
		if cl.runOrder[i].helperIndex != 0 && cl.runOrder[i].ss.moveType == MT_I {
			cl.runOrder[i].actionRun()
		}
	}
	// Run actions for remaining helpers
	for i := 0; i < len(cl.runOrder); i++ {
		if cl.runOrder[i].helperIndex != 0 {
			cl.runOrder[i].actionRun()
		}
	}
	// Finish performing character actions
	for i := 0; i < len(cl.runOrder); i++ {
		cl.runOrder[i].actionFinish()
	}
	// Update chars
	sys.charUpdate(cvmin, cvmax, highest, lowest, leftest, rightest)
}

func getAIInputs(player int) []byte {
	var ib InputBits
	ib.SetInputAI(player)
	return writeI32(int32(ib))
}

func (ib *InputBits) SetInputAI(in int) {
	*ib = InputBits(Btoi(sys.aiInput[in].U()) |
		Btoi(sys.aiInput[in].D())<<1 |
		Btoi(sys.aiInput[in].L())<<2 |
		Btoi(sys.aiInput[in].R())<<3 |
		Btoi(sys.aiInput[in].a())<<4 |
		Btoi(sys.aiInput[in].b())<<5 |
		Btoi(sys.aiInput[in].c())<<6 |
		Btoi(sys.aiInput[in].x())<<7 |
		Btoi(sys.aiInput[in].y())<<8 |
		Btoi(sys.aiInput[in].z())<<9 |
		Btoi(sys.aiInput[in].s())<<10 |
		Btoi(sys.aiInput[in].d())<<11 |
		Btoi(sys.aiInput[in].w())<<12 |
		Btoi(sys.aiInput[in].m())<<13)
}

type Fight struct {
	fin                          bool
	oldTeamLeader                [2]int
	oldWins                      [2]int32
	oldDraws                     int32
	oldStageVars                 Stage
	level                        []int32
	lvmul                        float64
	life, pow, gpow, spow, rlife []int32
	ivar                         [][]int32
	fvar                         [][]float32
	dialogue                     [][]string
	mapArray                     []map[string]float32
	remapSpr                     []RemapPreset
}

func (f *Fight) copyVar(pn int) {
	f.life[pn] = sys.chars[pn][0].life
	f.pow[pn] = sys.chars[pn][0].power
	f.gpow[pn] = sys.chars[pn][0].guardPoints
	f.spow[pn] = sys.chars[pn][0].dizzyPoints
	f.rlife[pn] = sys.chars[pn][0].redLife
	if len(f.ivar[pn]) < len(sys.chars[pn][0].ivar) {
		f.ivar[pn] = make([]int32, len(sys.chars[pn][0].ivar))
	}
	copy(f.ivar[pn], sys.chars[pn][0].ivar[:])
	if len(f.fvar[pn]) < len(sys.chars[pn][0].fvar) {
		f.fvar[pn] = make([]float32, len(sys.chars[pn][0].fvar))
	}
	copy(f.fvar[pn], sys.chars[pn][0].fvar[:])
	copy(f.dialogue[pn], sys.chars[pn][0].dialogue[:])
	f.mapArray[pn] = make(map[string]float32)
	for k, v := range sys.chars[pn][0].mapArray {
		f.mapArray[pn][k] = v
	}
	f.remapSpr[pn] = make(RemapPreset)
	for k, v := range sys.chars[pn][0].remapSpr {
		f.remapSpr[pn][k] = v
	}
	// Reset hitScale.
	sys.chars[pn][0].defaultHitScale = newHitScaleArray()
	sys.chars[pn][0].activeHitScale = make(map[int32][3]*HitScale)
	sys.chars[pn][0].nextHitScale = make(map[int32][3]*HitScale)

}

func (f *Fight) reset() {
	sys.wins, sys.draws = f.oldWins, f.oldDraws
	sys.teamLeader = f.oldTeamLeader
	for i, p := range sys.chars {
		if len(p) > 0 {
			p[0].life = f.life[i]
			p[0].power = f.pow[i]
			p[0].guardPoints = f.gpow[i]
			p[0].dizzyPoints = f.spow[i]
			p[0].redLife = f.rlife[i]
			copy(p[0].ivar[:], f.ivar[i])
			copy(p[0].fvar[:], f.fvar[i])
			copy(p[0].dialogue[:], f.dialogue[i])
			p[0].mapArray = make(map[string]float32)
			for k, v := range f.mapArray[i] {
				p[0].mapArray[k] = v
			}
			p[0].remapSpr = make(RemapPreset)
			for k, v := range f.remapSpr[i] {
				p[0].remapSpr[k] = v
			}

			// Reset hitScale
			p[0].defaultHitScale = newHitScaleArray()
			p[0].activeHitScale = make(map[int32][3]*HitScale)
			p[0].nextHitScale = make(map[int32][3]*HitScale)
		}
	}
	sys.stage.copyStageVars(&f.oldStageVars)
	sys.resetFrameTime()
	sys.nextRound()
	sys.roundResetFlg, sys.introSkipped = false, false
	sys.reloadFlg, sys.reloadStageFlg, sys.reloadLifebarFlg = false, false, false
	sys.cam.Update(sys.cam.startzoom, 0, 0)
}

func (f *Fight) endFight() {
	sys.oldNextAddTime = 1
	sys.nomusic = false
	sys.allPalFX.clear()
	sys.allPalFX.enable = false
	for i, p := range sys.chars {
		if len(p) > 0 {
			sys.playerClear(i, sys.matchOver() || (sys.tmode[i&1] == TM_Turns && p[0].life <= 0))
		}
	}
	sys.wincnt.update()
}

func (f *Fight) initChars() {
	// Initialize each character
	f.lvmul = math.Pow(2, 1.0/12)
	for i, p := range sys.chars {
		if len(p) > 0 {
			// Get max life, and adjust based on team mode
			var lm float32
			if p[0].ocd().lifeMax != -1 {
				lm = float32(p[0].ocd().lifeMax) * p[0].ocd().lifeRatio * sys.lifeMul
			} else {
				lm = float32(p[0].gi().data.life) * p[0].ocd().lifeRatio * sys.lifeMul
			}
			if p[0].teamside != -1 {
				switch sys.tmode[i&1] {
				case TM_Single:
					switch sys.tmode[(i+1)&1] {
					case TM_Simul, TM_Tag:
						lm *= sys.team1VS2Life
					case TM_Turns:
						if sys.numTurns[(i+1)&1] < sys.matchWins[(i+1)&1] && sys.lifeShare[i&1] {
							lm = lm * float32(sys.numTurns[(i+1)&1]) /
								float32(sys.matchWins[(i+1)&1])
						}
					}
				case TM_Simul, TM_Tag:
					switch sys.tmode[(i+1)&1] {
					case TM_Simul, TM_Tag:
						if sys.numSimul[(i+1)&1] < sys.numSimul[i&1] && sys.lifeShare[i&1] {
							lm = lm * float32(sys.numSimul[(i+1)&1]) / float32(sys.numSimul[i&1])
						}
					case TM_Turns:
						if sys.numTurns[(i+1)&1] < sys.numSimul[i&1]*sys.matchWins[(i+1)&1] && sys.lifeShare[i&1] {
							lm = lm * float32(sys.numTurns[(i+1)&1]) /
								float32(sys.numSimul[i&1]*sys.matchWins[(i+1)&1])
						}
					default:
						if sys.lifeShare[i&1] {
							lm /= float32(sys.numSimul[i&1])
						}
					}
				case TM_Turns:
					switch sys.tmode[(i+1)&1] {
					case TM_Single:
						if sys.matchWins[i&1] < sys.numTurns[i&1] && sys.lifeShare[i&1] {
							lm = lm * float32(sys.matchWins[i&1]) / float32(sys.numTurns[i&1])
						}
					case TM_Simul, TM_Tag:
						if sys.numSimul[(i+1)&1]*sys.matchWins[i&1] < sys.numTurns[i&1] && sys.lifeShare[i&1] {
							lm = lm * sys.team1VS2Life *
								float32(sys.numSimul[(i+1)&1]*sys.matchWins[i&1]) /
								float32(sys.numTurns[i&1])
						}
					case TM_Turns:
						if sys.numTurns[(i+1)&1] < sys.numTurns[i&1] && sys.lifeShare[i&1] {
							lm = lm * float32(sys.numTurns[(i+1)&1]) / float32(sys.numTurns[i&1])
						}
					}
				}
			}
			foo := math.Pow(f.lvmul, float64(-f.level[i]))
			p[0].lifeMax = Max(1, int32(math.Floor(foo*float64(lm))))

			if p[0].roundsExisted() > 0 {
				/* If character already existed for a round, presumably because of turns mode, just update life */
				p[0].life = Min(p[0].lifeMax, int32(math.Ceil(foo*float64(p[0].life))))
			} else if sys.round == 1 || sys.tmode[i&1] == TM_Turns {
				/* If round 1 or a new character in turns mode, initialize values */
				if p[0].ocd().life != -1 {
					p[0].life = Clamp(p[0].ocd().life, 0, p[0].lifeMax)
				} else {
					p[0].life = p[0].lifeMax
				}
				if sys.round == 1 {
					if sys.maxPowerMode {
						p[0].power = p[0].powerMax
					} else if p[0].ocd().power != -1 {
						p[0].power = Clamp(p[0].ocd().power, 0, p[0].powerMax)
					} else if !sys.consecutiveRounds || sys.consecutiveWins[0] == 0 {
						p[0].power = 0
					}
				}
				p[0].dialogue = []string{}
				p[0].mapArray = make(map[string]float32)
				for k, v := range p[0].mapDefault {
					p[0].mapArray[k] = v
				}
				p[0].remapSpr = make(RemapPreset)

				// Reset hitScale
				p[0].defaultHitScale = newHitScaleArray()
				p[0].activeHitScale = make(map[int32][3]*HitScale)
				p[0].nextHitScale = make(map[int32][3]*HitScale)
			}

			if p[0].ocd().guardPoints != -1 {
				p[0].guardPoints = Clamp(p[0].ocd().guardPoints, 0, p[0].guardPointsMax)
			} else {
				p[0].guardPoints = p[0].guardPointsMax
			}
			if p[0].ocd().dizzyPoints != -1 {
				p[0].dizzyPoints = Clamp(p[0].ocd().dizzyPoints, 0, p[0].dizzyPointsMax)
			} else {
				p[0].dizzyPoints = p[0].dizzyPointsMax
			}
			p[0].redLife = p[0].lifeMax
			f.copyVar(i)
		}
	}
}
func (f *Fight) initSuperMeter() {
	for i, p := range sys.chars {
		if len(p) > 0 {
			p[0].clear2()
			f.level[i] = sys.wincnt.getLevel(i)
			if sys.powerShare[i&1] && p[0].teamside != -1 {
				pmax := Max(sys.cgi[i&1].data.power, sys.cgi[i].data.power)
				for j := i & 1; j < MaxSimul*2; j += 2 {
					if len(sys.chars[j]) > 0 {
						sys.chars[j][0].powerMax = pmax
					}
				}
			}
		}
	}
}

func (f *Fight) initTeamsLevels() {
	minlv, maxlv := f.level[0], f.level[0]
	for i, lv := range f.level[1:] {
		if len(sys.chars[i+1]) > 0 {
			minlv = Min(minlv, lv)
			maxlv = Max(maxlv, lv)
		}
	}
	if minlv > 0 {
		for i := range f.level {
			f.level[i] -= minlv
		}
	} else if maxlv < 0 {
		for i := range f.level {
			f.level[i] -= maxlv
		}
	}
}
func NewFight() Fight {
	f := Fight{}
	f.oldStageVars.copyStageVars(sys.stage)
	f.life = make([]int32, len(sys.chars))
	f.pow = make([]int32, len(sys.chars))
	f.gpow = make([]int32, len(sys.chars))
	f.spow = make([]int32, len(sys.chars))
	f.rlife = make([]int32, len(sys.chars))
	f.ivar = make([][]int32, len(sys.chars))
	f.fvar = make([][]float32, len(sys.chars))
	f.dialogue = make([][]string, len(sys.chars))
	f.mapArray = make([]map[string]float32, len(sys.chars))
	f.remapSpr = make([]RemapPreset, len(sys.chars))
	f.level = make([]int32, len(sys.chars))
	return f
}
func readI32(b []byte) int32 {
	if len(b) < 4 {
		return 0
	}
	return int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16 | int32(b[3])<<24
}

func decodeInputs(buffer [][]byte) []InputBits {
	var inputs = make([]InputBits, len(buffer))
	for i, b := range buffer {
		inputs[i] = InputBits(readI32(b))
	}
	return inputs
}

// HACK: So you won't be playing eachothers characters
func reverseInputs(inputs []InputBits) []InputBits {
	for i, j := 0, len(inputs)-1; i < j; i, j = i+1, j-1 {
		inputs[i], inputs[j] = inputs[j], inputs[i]
	}
	return inputs
}

func writeI32(i32 int32) []byte {
	b := []byte{byte(i32), byte(i32 >> 8), byte(i32 >> 16), byte(i32 >> 24)}
	return b
}

func getInputs(player int) []byte {
	var ib InputBits
	ib.SetInput(player)
	return writeI32(int32(ib))
}

func (rs *RollbackSystem) roundState(s *System) int32 {
	switch {
	case s.postMatchFlg:
		return -1
	case s.intro > s.lifebar.ro.ctrl_time+1:
		return 0
	case s.lifebar.ro.cur == 0:
		return 1
	case s.intro >= 0 || s.finish == FT_NotYet:
		return 2
	case s.intro < -s.lifebar.ro.over_waittime:
		return 4
	default:
		return 3
	}
}
