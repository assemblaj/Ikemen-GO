package main

import (
	"github.com/go-gl/glfw/v3.2/glfw"
	"strings"
)

var netInput *NetInput
var fileInput *FileInput
var aiInput []AiInput
var keyConfig []*KeyConfig

type CommandKey byte

const (
	CK_B CommandKey = iota
	CK_D
	CK_F
	CK_U
	CK_DB
	CK_UB
	CK_DF
	CK_UF
	CK_nB
	CK_nD
	CK_nF
	CK_nU
	CK_nDB
	CK_nUB
	CK_nDF
	CK_nUF
	CK_Bs
	CK_Ds
	CK_Fs
	CK_Us
	CK_DBs
	CK_UBs
	CK_DFs
	CK_UFs
	CK_nBs
	CK_nDs
	CK_nFs
	CK_nUs
	CK_nDBs
	CK_nUBs
	CK_nDFs
	CK_nUFs
	CK_a
	CK_b
	CK_c
	CK_x
	CK_y
	CK_z
	CK_s
	CK_na
	CK_nb
	CK_nc
	CK_nx
	CK_ny
	CK_nz
	CK_ns
	CK_Last = CK_ns
)

type NetState int

const (
	NetStop NetState = iota
	NetPlaying
	NetEnd
	NetStoped
	NetError
)

var keySatate = make(map[glfw.Key]bool)

func keyCallback(_ *glfw.Window, key glfw.Key, _ int,
	action glfw.Action, _ glfw.ModifierKey) {
	switch action {
	case glfw.Release:
		keySatate[key] = false
	case glfw.Press:
		keySatate[key] = true
	}
}

var joystick = [...]glfw.Joystick{glfw.Joystick1, glfw.Joystick2,
	glfw.Joystick3, glfw.Joystick4, glfw.Joystick5, glfw.Joystick6,
	glfw.Joystick7, glfw.Joystick8, glfw.Joystick9, glfw.Joystick10,
	glfw.Joystick11, glfw.Joystick12, glfw.Joystick13, glfw.Joystick14,
	glfw.Joystick15, glfw.Joystick16}

func JoystickState(joy, button int) bool {
	if joy < 0 {
		return keySatate[glfw.Key(button)]
	}
	if joy >= len(joystick) {
		return false
	}
	if button < 0 {
		button = -button - 1
		axes := glfw.GetJoystickAxes(joystick[joy])
		if len(axes)*2 <= button {
			return false
		}
		switch button & 1 {
		case 0:
			return axes[button/2] < -0.1
		case 1:
			return axes[button/2] > 0.1
		}
	}
	btns := glfw.GetJoystickButtons(joystick[joy])
	if len(btns) <= button {
		return false
	}
	return btns[button] != 0
}

type KeyConfig struct{ Joy, U, D, L, R, A, B, C, X, Y, Z, S int }
type InputBits int32

const (
	IB_U InputBits = 1 << iota
	IB_D
	IB_L
	IB_R
	IB_A
	IB_B
	IB_C
	IB_X
	IB_Y
	IB_Z
	IB_S
	IB_anybutton = IB_A | IB_B | IB_C | IB_X | IB_Y | IB_Z
)

func (ib *InputBits) SetInput(in int) {
	if 0 <= in && in < len(keyConfig) {
		*ib = InputBits(Btoi(JoystickState(keyConfig[in].Joy, keyConfig[in].U)) |
			Btoi(JoystickState(keyConfig[in].Joy, keyConfig[in].D))<<1 |
			Btoi(JoystickState(keyConfig[in].Joy, keyConfig[in].L))<<2 |
			Btoi(JoystickState(keyConfig[in].Joy, keyConfig[in].R))<<3 |
			Btoi(JoystickState(keyConfig[in].Joy, keyConfig[in].A))<<4 |
			Btoi(JoystickState(keyConfig[in].Joy, keyConfig[in].B))<<5 |
			Btoi(JoystickState(keyConfig[in].Joy, keyConfig[in].C))<<6 |
			Btoi(JoystickState(keyConfig[in].Joy, keyConfig[in].X))<<7 |
			Btoi(JoystickState(keyConfig[in].Joy, keyConfig[in].Y))<<8 |
			Btoi(JoystickState(keyConfig[in].Joy, keyConfig[in].Z))<<9 |
			Btoi(JoystickState(keyConfig[in].Joy, keyConfig[in].S))<<10)
	}
}

type CommandKeyRemap struct {
	a, b, c, x, y, z, s, na, nb, nc, nx, ny, nz, ns CommandKey
}

func NewCommandKeyRemap() *CommandKeyRemap {
	return &CommandKeyRemap{CK_a, CK_b, CK_c, CK_x, CK_y, CK_z, CK_s,
		CK_na, CK_nb, CK_nc, CK_nx, CK_ny, CK_nz, CK_ns}
}

type CommandBuffer struct {
	Bb, Db, Fb, Ub             int32
	ab, bb, cb, xb, yb, zb, sb int32
	B, D, F, U                 int8
	a, b, c, x, y, z, s        int8
}

func newCommandBuffer() *CommandBuffer {
	return &CommandBuffer{B: -1, D: -1, F: -1, U: -1,
		a: -1, b: -1, c: -1, x: -1, y: -1, z: -1, s: -1}
}
func (__ *CommandBuffer) Input(B, D, F, U, a, b, c, x, y, z, s bool) {
	if (B && !F) != (__.B > 0) {
		__.Bb = 0
		__.B *= -1
	}
	__.Bb += int32(__.B)
	if (D && !U) != (__.D > 0) {
		__.Db = 0
		__.D *= -1
	}
	__.Db += int32(__.D)
	if (F && !B) != (__.F > 0) {
		__.Fb = 0
		__.F *= -1
	}
	__.Fb += int32(__.F)
	if (U && !D) != (__.U > 0) {
		__.Ub = 0
		__.U *= -1
	}
	__.Ub += int32(__.U)
	if a != (__.a > 0) {
		__.ab = 0
		__.a *= -1
	}
	__.ab += int32(__.a)
	if b != (__.b > 0) {
		__.bb = 0
		__.b *= -1
	}
	__.bb += int32(__.b)
	if c != (__.c > 0) {
		__.cb = 0
		__.c *= -1
	}
	__.cb += int32(__.c)
	if x != (__.x > 0) {
		__.xb = 0
		__.x *= -1
	}
	__.xb += int32(__.x)
	if y != (__.y > 0) {
		__.yb = 0
		__.y *= -1
	}
	__.yb += int32(__.y)
	if z != (__.z > 0) {
		__.zb = 0
		__.z *= -1
	}
	__.zb += int32(__.z)
	if s != (__.s > 0) {
		__.sb = 0
		__.s *= -1
	}
	__.sb += int32(__.s)
}
func (__ *CommandBuffer) InputBits(ib InputBits, f int32) {
	var B, F bool
	if f < 0 {
		B, F = ib&IB_R != 0, ib&IB_L != 0
	} else {
		B, F = ib&IB_L != 0, ib&IB_R != 0
	}
	__.Input(B, ib&IB_D != 0, F, ib&IB_U != 0, ib&IB_A != 0, ib&IB_B != 0,
		ib&IB_C != 0, ib&IB_X != 0, ib&IB_Y != 0, ib&IB_Z != 0, ib&IB_S != 0)
}
func (__ *CommandBuffer) State(ck CommandKey) int32 {
	switch ck {
	case CK_B:
		return Min(-Max(__.Db, __.Ub), __.Bb)
	case CK_D:
		return Min(-Max(__.Bb, __.Fb), __.Db)
	case CK_F:
		return Min(-Max(__.Db, __.Ub), __.Fb)
	case CK_U:
		return Min(-Max(__.Bb, __.Fb), __.Ub)
	case CK_DB:
		return Min(__.Db, __.Bb)
	case CK_UB:
		return Min(__.Ub, __.Bb)
	case CK_DF:
		return Min(__.Db, __.Fb)
	case CK_UF:
		return Min(__.Ub, __.Fb)
	case CK_Bs:
		return __.Bb
	case CK_Ds:
		return __.Db
	case CK_Fs:
		return __.Fb
	case CK_Us:
		return __.Ub
	case CK_DBs:
		return Min(-Max(__.Ub, __.Fb), Max(__.Db, __.Bb))
	case CK_UBs:
		return Min(-Max(__.Db, __.Fb), Max(__.Ub, __.Bb))
	case CK_DFs:
		return Min(-Max(__.Ub, __.Bb), Max(__.Db, __.Fb))
	case CK_UFs:
		return Min(-Max(__.Db, __.Bb), Max(__.Ub, __.Fb))
	case CK_a:
		return __.ab
	case CK_b:
		return __.bb
	case CK_c:
		return __.cb
	case CK_x:
		return __.xb
	case CK_y:
		return __.yb
	case CK_z:
		return __.zb
	case CK_s:
		return __.sb
	case CK_nB:
		return -Min(-Max(__.Db, __.Ub), __.Bb)
	case CK_nD:
		return -Min(-Max(__.Bb, __.Fb), __.Db)
	case CK_nF:
		return -Min(-Max(__.Db, __.Ub), __.Fb)
	case CK_nU:
		return -Min(-Max(__.Bb, __.Fb), __.Ub)
	case CK_nDB:
		return -Min(__.Db, __.Bb)
	case CK_nUB:
		return -Min(__.Ub, __.Bb)
	case CK_nDF:
		return -Min(__.Db, __.Fb)
	case CK_nUF:
		return -Min(__.Ub, __.Fb)
	case CK_nBs:
		return -__.Bb
	case CK_nDs:
		return -__.Db
	case CK_nFs:
		return -__.Fb
	case CK_nUs:
		return -__.Ub
	case CK_nDBs:
		return -Min(-Max(__.Ub, __.Fb), Max(__.Db, __.Bb))
	case CK_nUBs:
		return -Min(-Max(__.Db, __.Fb), Max(__.Ub, __.Bb))
	case CK_nDFs:
		return -Min(-Max(__.Ub, __.Bb), Max(__.Db, __.Fb))
	case CK_nUFs:
		return -Min(-Max(__.Db, __.Bb), Max(__.Ub, __.Fb))
	case CK_na:
		return -__.ab
	case CK_nb:
		return -__.bb
	case CK_nc:
		return -__.cb
	case CK_nx:
		return -__.xb
	case CK_ny:
		return -__.yb
	case CK_nz:
		return -__.zb
	case CK_ns:
		return -__.sb
	}
	return 0
}
func (__ *CommandBuffer) State2(ck CommandKey) int32 {
	f := func(a, b, c int32) int32 {
		switch {
		case a > 0:
			return -Max(b, c)
		case b > 0:
			return -Max(a, c)
		case c > 0:
			return -Max(a, b)
		}
		return -Max(a, b, c)
	}
	switch ck {
	case CK_Bs:
		if __.Bb < 0 {
			return __.Bb
		}
		return Min(Abs(__.Bb), Abs(__.Db), Abs(__.Ub))
	case CK_Ds:
		if __.Db < 0 {
			return __.Db
		}
		return Min(Abs(__.Db), Abs(__.Bb), Abs(__.Fb))
	case CK_Fs:
		if __.Fb < 0 {
			return __.Fb
		}
		return Min(Abs(__.Fb), Abs(__.Db), Abs(__.Ub))
	case CK_Us:
		if __.Ub < 0 {
			return __.Ub
		}
		return Min(Abs(__.Ub), Abs(__.Bb), Abs(__.Fb))
	case CK_DBs:
		if s := __.State(CK_DBs); s < 0 {
			return s
		}
		return Min(Abs(__.Db), Abs(__.Bb))
	case CK_UBs:
		if s := __.State(CK_UBs); s < 0 {
			return s
		}
		return Min(Abs(__.Ub), Abs(__.Bb))
	case CK_DFs:
		if s := __.State(CK_DFs); s < 0 {
			return s
		}
		return Min(Abs(__.Db), Abs(__.Fb))
	case CK_UFs:
		if s := __.State(CK_UFs); s < 0 {
			return s
		}
		return Min(Abs(__.Ub), Abs(__.Fb))
	case CK_nBs:
		return f(__.State(CK_B), __.State(CK_UB), __.State(CK_DB))
	case CK_nDs:
		return f(__.State(CK_D), __.State(CK_DB), __.State(CK_DF))
	case CK_nFs:
		return f(__.State(CK_F), __.State(CK_DF), __.State(CK_UF))
	case CK_nUs:
		return f(__.State(CK_U), __.State(CK_UB), __.State(CK_UF))
	case CK_nDBs:
		return f(__.State(CK_DB), __.State(CK_D), __.State(CK_B))
	case CK_nUBs:
		return f(__.State(CK_UB), __.State(CK_U), __.State(CK_B))
	case CK_nDFs:
		return f(__.State(CK_DF), __.State(CK_D), __.State(CK_F))
	case CK_nUFs:
		return f(__.State(CK_UF), __.State(CK_U), __.State(CK_F))
	}
	return __.State(ck)
}
func (__ *CommandBuffer) LastDirectionTime() int32 {
	return Min(Abs(__.Bb), Abs(__.Db), Abs(__.Fb), Abs(__.Ub))
}
func (__ *CommandBuffer) LastChangeTime() int32 {
	return Min(__.LastDirectionTime(), Abs(__.ab), Abs(__.bb), Abs(__.cb),
		Abs(__.xb), Abs(__.yb), Abs(__.zb), Abs(__.sb))
}

type NetBuffer struct {
	buf              [32]InputBits
	curT, inpT, senT int
}
type NetInput struct{ buf []NetBuffer }
type FileInput struct{ ib []InputBits }
type AiInput struct {
	dir, dt, at, bt, ct, xt, yt, zt, st int32
}

func (__ *AiInput) Update() {
	if introTime != 0 {
		__.dt, __.at, __.bt, __.ct = 0, 0, 0, 0
		__.xt, __.yt, __.zt, __.st = 0, 0, 0, 0
		return
	}
	var osu, hanasu int32 = 15, 60
	dec := func(t *int32) bool {
		(*t)--
		if *t <= 0 {
			if Rand(1, osu) == 1 {
				*t = Rand(1, hanasu)
				return true
			}
			*t = 0
		}
		return false
	}
	if dec(&__.dt) {
		__.dir = Rand(0, 7)
	}
	osu, hanasu = 30, 30
	dec(&__.at)
	dec(&__.bt)
	dec(&__.ct)
	dec(&__.xt)
	dec(&__.yt)
	dec(&__.zt)
	osu = 3600
	dec(&__.st)
}
func (__ *AiInput) L() bool {
	return __.dt != 0 && (__.dir == 5 || __.dir == 6 || __.dir == 7)
}
func (__ *AiInput) R() bool {
	return __.dt != 0 && (__.dir == 1 || __.dir == 2 || __.dir == 3)
}
func (__ *AiInput) U() bool {
	return __.dt != 0 && (__.dir == 7 || __.dir == 0 || __.dir == 1)
}
func (__ *AiInput) D() bool {
	return __.dt != 0 && (__.dir == 3 || __.dir == 4 || __.dir == 5)
}
func (__ *AiInput) A() bool {
	return __.at != 0
}
func (__ *AiInput) B() bool {
	return __.bt != 0
}
func (__ *AiInput) C() bool {
	return __.ct != 0
}
func (__ *AiInput) X() bool {
	return __.xt != 0
}
func (__ *AiInput) Y() bool {
	return __.yt != 0
}
func (__ *AiInput) Z() bool {
	return __.zt != 0
}
func (__ *AiInput) S() bool {
	return __.st != 0
}

type cmdElem struct {
	key                       []CommandKey
	tametime                  int32
	slash, greater, direction bool
}

func (ce *cmdElem) IsDirection() bool {
	return !ce.slash && len(ce.key) == 1 && ce.key[0] < CK_a
}
func (ce *cmdElem) IsDToB(next cmdElem) bool {
	if next.slash {
		return false
	}
	for _, k := range ce.key {
		if k >= CK_a {
			return false
		}
	}
	if len(ce.key) != len(next.key) {
		return true
	}
	for i, k := range ce.key {
		if k != next.key[i] && ((CK_nB <= k && k <= CK_nUF) ||
			(CK_nBs <= k && k <= CK_nUFs) ||
			(CK_nB <= next.key[i] && next.key[i] <= CK_nUF) ||
			(CK_nBs <= next.key[i] && next.key[i] <= CK_nUFs)) {
			return true
		}
	}
	return false
}

type Command struct {
	name                string
	hold                [][]CommandKey
	held                []bool
	cmd                 []cmdElem
	cmdi, tamei         int
	time, cur           int32
	buftime, curbuftime int32
}

func newCommand() *Command { return &Command{tamei: -1, time: 1, buftime: 1} }
func ReadCommand(cmdstr string) *Command {
	c := newCommand()
	cmd := strings.Split(cmdstr, ",")
	for _, cestr := range cmd {
		if len(c.cmd) > 0 && c.cmd[len(c.cmd)-1].slash {
			c.hold = append(c.hold, c.cmd[len(c.cmd)-1].key)
			c.cmd[len(c.cmd)-1] = cmdElem{tametime: 1}
		} else {
			c.cmd = append(c.cmd, cmdElem{tametime: 1})
		}
		ce := &c.cmd[len(c.cmd)-1]
		cestr = strings.TrimSpace(cestr)
		getChar := func() rune {
			if len(cestr) > 0 {
				return rune(cestr[0])
			}
			return rune(-1)
		}
		nextChar := func() rune {
			if len(cestr) > 0 {
				cestr = strings.TrimSpace(cestr[1:])
			}
			return getChar()
		}
		tilde := false
		switch getChar() {
		case '>':
			ce.greater = true
			r := nextChar()
			if r == '/' {
				ce.slash = true
				nextChar()
				break
			} else if r == '~' {
			} else {
				break
			}
			fallthrough
		case '~':
			tilde = true
			n := int32(0)
			for r := nextChar(); '0' <= r && r <= '9'; r = nextChar() {
				n = n*10 + int32(r-'0')
			}
			if n > 0 {
				ce.tametime = n
			}
		case '/':
			ce.slash = true
			nextChar()
		}
		for len(cestr) > 0 {
			switch getChar() {
			case 'B':
				if tilde {
					ce.key = append(ce.key, CK_nB)
				} else {
					ce.key = append(ce.key, CK_B)
				}
				tilde = false
			case 'D':
				if len(cestr) > 1 && cestr[1] == 'B' {
					nextChar()
					if tilde {
						ce.key = append(ce.key, CK_nDB)
					} else {
						ce.key = append(ce.key, CK_DB)
					}
				} else if len(cestr) > 1 && cestr[1] == 'F' {
					nextChar()
					if tilde {
						ce.key = append(ce.key, CK_nDF)
					} else {
						ce.key = append(ce.key, CK_DF)
					}
				} else {
					if tilde {
						ce.key = append(ce.key, CK_nD)
					} else {
						ce.key = append(ce.key, CK_D)
					}
				}
				tilde = false
			case 'F':
				if tilde {
					ce.key = append(ce.key, CK_nF)
				} else {
					ce.key = append(ce.key, CK_F)
				}
				tilde = false
			case 'U':
				if len(cestr) > 1 && cestr[1] == 'B' {
					nextChar()
					if tilde {
						ce.key = append(ce.key, CK_nUB)
					} else {
						ce.key = append(ce.key, CK_UB)
					}
				} else if len(cestr) > 1 && cestr[1] == 'F' {
					nextChar()
					if tilde {
						ce.key = append(ce.key, CK_nUF)
					} else {
						ce.key = append(ce.key, CK_UF)
					}
				} else {
					if tilde {
						ce.key = append(ce.key, CK_nU)
					} else {
						ce.key = append(ce.key, CK_U)
					}
				}
				tilde = false
			case 'a':
				if tilde {
					ce.key = append(ce.key, CK_na)
				} else {
					ce.key = append(ce.key, CK_a)
				}
				tilde = false
			case 'b':
				if tilde {
					ce.key = append(ce.key, CK_nb)
				} else {
					ce.key = append(ce.key, CK_b)
				}
				tilde = false
			case 'c':
				if tilde {
					ce.key = append(ce.key, CK_nc)
				} else {
					ce.key = append(ce.key, CK_c)
				}
				tilde = false
			case 'x':
				if tilde {
					ce.key = append(ce.key, CK_nx)
				} else {
					ce.key = append(ce.key, CK_x)
				}
				tilde = false
			case 'y':
				if tilde {
					ce.key = append(ce.key, CK_ny)
				} else {
					ce.key = append(ce.key, CK_y)
				}
				tilde = false
			case 'z':
				if tilde {
					ce.key = append(ce.key, CK_nz)
				} else {
					ce.key = append(ce.key, CK_z)
				}
				tilde = false
			case 's':
				if tilde {
					ce.key = append(ce.key, CK_ns)
				} else {
					ce.key = append(ce.key, CK_s)
				}
				tilde = false
			case '$':
				switch nextChar() {
				case 'B':
					if tilde {
						ce.key = append(ce.key, CK_nBs)
					} else {
						ce.key = append(ce.key, CK_Bs)
					}
					tilde = false
				case 'D':
					if len(cestr) > 1 && cestr[1] == 'B' {
						nextChar()
						if tilde {
							ce.key = append(ce.key, CK_nDBs)
						} else {
							ce.key = append(ce.key, CK_DBs)
						}
					} else if len(cestr) > 1 && cestr[1] == 'F' {
						nextChar()
						if tilde {
							ce.key = append(ce.key, CK_nDFs)
						} else {
							ce.key = append(ce.key, CK_DFs)
						}
					} else {
						if tilde {
							ce.key = append(ce.key, CK_nDs)
						} else {
							ce.key = append(ce.key, CK_Ds)
						}
					}
					tilde = false
				case 'F':
					if tilde {
						ce.key = append(ce.key, CK_nFs)
					} else {
						ce.key = append(ce.key, CK_Fs)
					}
					tilde = false
				case 'U':
					if len(cestr) > 1 && cestr[1] == 'B' {
						nextChar()
						if tilde {
							ce.key = append(ce.key, CK_nUBs)
						} else {
							ce.key = append(ce.key, CK_UBs)
						}
					} else if len(cestr) > 1 && cestr[1] == 'F' {
						nextChar()
						if tilde {
							ce.key = append(ce.key, CK_nUFs)
						} else {
							ce.key = append(ce.key, CK_UFs)
						}
					} else {
						if tilde {
							ce.key = append(ce.key, CK_nUs)
						} else {
							ce.key = append(ce.key, CK_Us)
						}
					}
					tilde = false
				default:
					// error
					continue
				}
			case '~':
				tilde = true
			case '+':
			default:
				// error
			}
			nextChar()
		}
		if len(c.cmd) >= 2 && ce.IsDirection() &&
			c.cmd[len(c.cmd)-2].IsDirection() {
			ce.direction = true
		}
	}
	if c.cmd[len(c.cmd)-1].slash {
		c.hold = append(c.hold, c.cmd[len(c.cmd)-1].key)
	}
	c.held = make([]bool, len(c.hold))
	return c
}
func (c *Command) Clear() {
	c.cmdi, c.tamei, c.cur, c.curbuftime = 0, -1, 0, 0
	for i := range c.held {
		c.held[i] = false
	}
}
func (c *Command) bufTest(cbuf *CommandBuffer, ai bool,
	holdTemp *[CK_Last + 1]bool) bool {
	anyHeld, notHeld := false, 0
	if len(c.hold) > 0 && !ai {
		if holdTemp == nil {
			holdTemp = &[CK_Last + 1]bool{}
			for i := range *holdTemp {
				(*holdTemp)[i] = true
			}
		}
		allHold := true
		for i, h := range c.hold {
			func() {
				for _, k := range h {
					ks := cbuf.State(k)
					if ks == 1 && (c.cmdi > 0 || len(c.hold) > 1) && !c.held[i] &&
						(*holdTemp)[int(k)] {
						c.held[i], (*holdTemp)[int(k)] = true, false
					}
					if ks > 0 {
						return
					}
				}
				allHold = false
			}()
			if c.held[i] {
				anyHeld = true
			} else {
				notHeld += 1
			}
		}
		if c.cmdi == len(c.cmd)-1 && (!allHold || notHeld > 1) {
			return anyHeld || c.cmdi > 0
		}
	}
	if !ai && c.cmd[c.cmdi].slash {
		if c.cmdi > 0 {
			if notHeld == 1 {
				if len(c.cmd[c.cmdi-1].key) != 1 {
					return false
				}
				if CK_a <= c.cmd[c.cmdi-1].key[0] && c.cmd[c.cmdi-1].key[0] <= CK_s {
					ks := cbuf.State(c.cmd[c.cmdi-1].key[0])
					if 0 < ks && ks <= cbuf.LastDirectionTime() {
						return true
					}
				}
			} else if len(c.cmd[c.cmdi-1].key) > 1 {
				for _, k := range c.cmd[c.cmdi-1].key {
					if CK_a <= k && k <= CK_s && cbuf.State(k) > 0 {
						return false
					}
				}
			}
		}
		c.cmdi++
		return true
	}
	fail := func() bool {
		if c.cmdi == 0 {
			return anyHeld
		}
		if !ai && (c.cmd[c.cmdi].greater || c.cmd[c.cmdi].direction) {
			var t int32
			if c.cmd[c.cmdi].greater {
				t = cbuf.LastChangeTime()
			} else {
				t = cbuf.LastDirectionTime()
			}
			for _, k := range c.cmd[c.cmdi-1].key {
				if cbuf.State2(k) == t {
					return true
				}
			}
			c.Clear()
			return c.bufTest(cbuf, ai, holdTemp)
		}
		return true
	}
	if c.tamei != c.cmdi {
		if c.cmd[c.cmdi].tametime > 1 {
			for _, k := range c.cmd[c.cmdi].key {
				ks:=cbuf.State(k)
				if ks>0{return ai}
				if func() bool {
					if ai {
						return Rand(0, c.cmd[c.cmdi].tametime) != 0
					}
					return -ks < c.cmd[c.cmdi].tametime
				}() {
					return anyHeld || c.cmdi > 0
				}
			}
			c.tamei = c.cmdi
		} else if c.cmdi > 0 && len(c.cmd[c.cmdi-1].key) == 1 &&
			len(c.cmd[c.cmdi].key) == 1 && c.cmd[c.cmdi-1].key[0] < CK_Bs &&
			c.cmd[c.cmdi].key[0] < CK_nB && (c.cmd[c.cmdi-1].key[0]-
			c.cmd[c.cmdi].key[0])&7 == 0 {
			if cbuf.B < 0 && cbuf.D < 0 && cbuf.F < 0 && cbuf.U < 0 {
				c.tamei = c.cmdi
			} else {
				return fail()
			}
		}
	}
	foo := false
	for _, k := range c.cmd[c.cmdi-1].key {
		n := cbuf.State2(k)
		if c.cmd[c.cmdi].slash {
			foo = foo || n > 0
		} else if n < 1 || 7 < n {
			return fail()
		} else {
			foo = foo || n == 1
		}
	}
	if !foo {
		return fail()
	}
	c.cmdi++
	if c.cmdi < len(c.cmd) && c.cmd[c.cmdi-1].IsDToB(c.cmd[c.cmdi]) {
		return c.bufTest(cbuf, ai, holdTemp)
	}
	return true
}
func (c *Command) Step(cbuf *CommandBuffer, ai, hitpause bool, buftime int32) {
	if !hitpause && c.curbuftime > 0 {
		c.curbuftime--
	}
	if len(c.cmd) == 0 {
		return
	}
	ocbt := c.curbuftime
	defer func() {
		if c.curbuftime < ocbt {
			c.curbuftime = ocbt
		}
	}()
	var holdTemp *[CK_Last + 1]bool
	if cbuf == nil || !c.bufTest(cbuf, ai, holdTemp) {
		foo := c.tamei == 0 && c.cmdi == 0
		c.Clear()
		if foo {
			c.tamei = 0
		}
		return
	}
	if c.cmdi == 1 && c.cmd[0].slash {
		c.cur = 0
	} else {
		c.cur++
	}
	complete := c.cmdi == len(c.cmd)
	if !complete && (ai || c.cur <= c.time) {
		return
	}
	c.Clear()
	if complete {
		c.curbuftime = c.buftime + buftime
	}
}