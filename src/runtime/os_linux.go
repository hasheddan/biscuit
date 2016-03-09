// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "unsafe"

func futex(addr unsafe.Pointer, op int32, val uint32, ts, addr2 unsafe.Pointer, val3 uint32) int32
func clone(flags int32, stk, mm, gg, fn unsafe.Pointer) int32
func rt_sigaction(sig uintptr, new, old unsafe.Pointer, size uintptr) int32
func sigaltstack(new, old unsafe.Pointer)
func setitimer(mode int32, new, old unsafe.Pointer)
func rtsigprocmask(sig int32, new, old unsafe.Pointer, size int32)
func getrlimit(kind int32, limit unsafe.Pointer) int32
func raise(sig int32)
func sched_getaffinity(pid, len uintptr, buf *uintptr) int32
func trapsched_m(*g)
func trapinit_m(*g)

func Ap_setup(int)
func Cli()
func Cpuid(uint32, uint32) (uint32, uint32, uint32, uint32)
func Install_traphandler(func(tf *[24]int))
func Invlpg(unsafe.Pointer)
func Kpmap() *[512]int
func Kpmap_p() int
func Lcr3(uintptr)
func Inl(int) int
func Insl(int, unsafe.Pointer, int)
func Outb(uint16, uint8)
func Inb(uint16) uint
func Outw(int, int)
func Outl(int, int)
func Outsl(int, unsafe.Pointer, int)
func Pmsga(*uint8, int, int8)
func Pnum(int)
func Kreset()
func Ktime() int
func Rdtsc() uint64
func Rcr2() uintptr
func Rcr3() uintptr
func Rcr4() uintptr
func Sgdt(*uintptr)
func Sidt(*uintptr)
func Sti()
func Vtop(*[512]int) int

func Crash()
func Fnaddr(func()) uintptr
func Fnaddri(func(int)) uintptr
func Trapwake()

func htpause()
func finit()
func fs_null()
func fxsave(*[fxwords]uintptr)
func gs_null()
func lgdt(pdesc_t)
func lidt(pdesc_t)
func cli()
func sti()
func ltr(uint)
func rcr0() uintptr
func rcr4() uintptr
func lcr0(uintptr)
func lcr4(uintptr)
func clone_call(uintptr)
func cpu_halt(uintptr)
func fxrstor(*[FXREGS]uintptr)
func trapret(*[TFSIZE]uintptr, uintptr)
func mktrap(int)
func stackcheck()

// we have to carefully write go code that may be executed early (during boot)
// or in interrupt context. such code cannot allocate or call functions that
// that have the stack splitting prologue. the following is a list of go code
// that could result in code that allocates or calls a function with a stack
// splitting prologue.
// - function with interface argument (calls convT2E)
// - taking address of stack variable (may allocate)
// - using range to iterate over a string (calls stringiter*)

type cpu_t struct {
	this		uint
	mythread	*thread_t
	rsp		uintptr
	num		uint
	pmap		*[512]int
	pms		[]*[512]int
	//pid		uintptr
}

var Cpumhz uint
var Pspercycle uint

const MAXCPUS int = 32

var cpus [MAXCPUS]cpu_t

type tuser_t struct {
	tf	uintptr
	fxbuf	uintptr
}

type prof_t struct {
	enabled		int
	totaltime	int
	stampstart	int
}

// XXX rearrange these for better spatial locality; p_pmap should probably be
// near front
type thread_t struct {
	tf		[TFSIZE]uintptr
	fx		[FXREGS]uintptr
	user		tuser_t
	sigtf		[TFSIZE]uintptr
	sigfx		[FXREGS]uintptr
	sigstatus	int
	siglseepfor	int
	status		int
	doingsig	int
	sigstack	uintptr
	prof		prof_t
	sleepfor	int
	sleepret	int
	futaddr		uintptr
	p_pmap		uintptr
	//_pad		int
}

// XXX fix these misleading names
const(
  TFSIZE       = 24
  FXREGS       = 64
  TFREGS       = 17
  TF_SYSRSP    = 0
  TF_FSBASE    = 1
  TF_R8        = 9
  TF_RBP       = 10
  TF_RSI       = 11
  TF_RDI       = 12
  TF_RDX       = 13
  TF_RCX       = 14
  TF_RBX       = 15
  TF_RAX       = 16
  TF_TRAP      = TFREGS
  TF_RIP       = TFREGS + 2
  TF_CS        = TFREGS + 3
  TF_RSP       = TFREGS + 5
  TF_SS        = TFREGS + 6
  TF_RFLAGS    = TFREGS + 4
    TF_FL_IF     uintptr = 1 << 9
)

func Pushcli() int
func Popcli(int)
func _Gscpu() *cpu_t
func Rdmsr(int) int
func Wrmsr(int, int)
func _Userrun(*[24]int, bool) (int, int)

func Cprint(byte, int)

//go:nosplit
func Gscpu() *cpu_t {
	if rflags() & TF_FL_IF != 0 {
		G_pancake("must not be interruptible", 0)
	}
	return _Gscpu()
}

func Userrun(tf *[24]int, fxbuf *[64]int, pmap *[512]int, p_pmap uintptr,
    pms []*[512]int, fastret bool) (int, int) {

	// {enter,exit}syscall() may not be worth the overhead. i believe the
	// only benefit for biscuit is that cpus running in the kernel could GC
	// while other cpus execute user programs.
	entersyscall()
	fl := Pushcli()
	cpu := Gscpu()
	ct := cpu.mythread

	// set shadow pointers for user pmap so it isn't free'd out from under
	// us if the process terminates soon
	cpu.pmap = pmap
	cpu.pms = pms
	//cpu.pid = uintptr(pid)
	if Rcr3() != p_pmap {
		Lcr3(p_pmap)
	}

	// if doing a fast return after a syscall, we need to restore some user
	// state manually
	ia32_fs_base := 0xc0000100
	kfsbase := Rdmsr(ia32_fs_base)
	Wrmsr(ia32_fs_base, tf[TF_FSBASE])

	// we only save/restore SSE registers on cpu exception/interrupt, not
	// during syscall exit/return. this is OK since sys5ABI defines the SSE
	// registers to be caller-saved.
	ct.user.tf = uintptr(unsafe.Pointer(tf))
	ct.user.fxbuf = uintptr(unsafe.Pointer(fxbuf))
	intno, aux := _Userrun(tf, fastret)

	Wrmsr(ia32_fs_base, kfsbase)
	ct.user.tf = 0
	ct.user.fxbuf = 0
	Popcli(fl)
	exitsyscall()
	return intno, aux
}

// caller must have interrupts cleared
//go:nosplit
func shadow_clear() {
	cpu := Gscpu()
	cpu.pmap = nil
	cpu.pms = nil
}

type nmiprof_t struct {
	buf		[]uintptr
	bufidx		uint64
	LVTmask		bool
	evtsel		int
	evtmin		uint
	evtmax		uint
}

var _nmibuf [4096]uintptr
var nmiprof = nmiprof_t{buf: _nmibuf[:]}

func SetNMI(mask bool, evtsel int, min, max uint) {
	nmiprof.LVTmask = mask
	nmiprof.evtsel = evtsel
	nmiprof.evtmin = min
	nmiprof.evtmax = max
}

func TakeNMIBuf() ([]uintptr, bool) {
	ret := nmiprof.buf
	l := int(nmiprof.bufidx)
	if l > len(ret) {
		l = len(ret)
	}
	ret = ret[:l]
	full := false
	if l == len(_nmibuf) {
		full = true
	}

	nmiprof.bufidx = 0
	return ret, full
}

var _seed uint

//go:nosplit
func dumrand(low, high uint) uint {
	ra := high - low
	if ra == 0 {
		return low
	}
	_seed = _seed * 1103515245 + 12345
	ret := _seed & 0x7fffffffffffffff
	return low + (ret % ra)
}

//go:nosplit
func _consumelbr() {
	lastbranch_tos := 0x1c9
	lastbranch_0_from_ip := 0x680
	lastbranch_0_to_ip := 0x6c0

	last := Rdmsr(lastbranch_tos) & 0xf
	// XXX stacklen
	l := 16 * 2
	l++
	idx := int(xadd64(&nmiprof.bufidx, int64(l)))
	idx -= l
	for i := 0; i < 16; i++ {
		cur := (last - i)
		if cur < 0 {
			cur += 16
		}
		from := uintptr(Rdmsr(lastbranch_0_from_ip + cur))
		to := uintptr(Rdmsr(lastbranch_0_to_ip + cur))
		Wrmsr(lastbranch_0_from_ip + cur, 0)
		Wrmsr(lastbranch_0_to_ip + cur, 0)
		if idx + 2*i + 1 >= len(nmiprof.buf) {
			Cprint('!', 1)
			break
		}
		nmiprof.buf[idx+2*i] = from
		nmiprof.buf[idx+2*i+1] = to
	}
	idx += l - 1
	if idx < len(nmiprof.buf) {
		nmiprof.buf[idx] = ^uintptr(0)
	}
}

//go:nosplit
func _lbrreset(en bool) {
	ia32_debugctl := 0x1d9
	if !en {
		Wrmsr(ia32_debugctl, 0)
		return
	}

	// enable last branch records. filter every branch but to direct
	// calls/jmps (sandybridge onward has better filtering)
	lbr_select := 0x1c8
	jcc := 1 << 2
	indjmp := 1 << 6
	//reljmp := 1 << 7
	farbr := 1 << 8
	dv := jcc | farbr | indjmp
	Wrmsr(lbr_select, dv)

	freeze_lbrs_on_pmi := 1 << 11
	lbrs := 1 << 0
	dv = lbrs | freeze_lbrs_on_pmi
	Wrmsr(ia32_debugctl, dv)
}

//go:nosplit
func perfgather(tf *[TFSIZE]uintptr) {
	idx := xadd64(&nmiprof.bufidx, 1) - 1
	if idx < uint64(len(nmiprof.buf)) {
		//nmiprof.buf[idx] = tf[TF_RIP]
		//pid := Gscpu().pid
		//v := tf[TF_RIP] | (pid << 56)
		v := tf[TF_RIP]
		nmiprof.buf[idx] = v
	}
	//_consumelbr()
}

//go:nosplit
func perfmask() {
	lapaddr := 0xfee00000
	lap := (*[PGSIZE/4]uint32)(unsafe.Pointer(uintptr(lapaddr)))

	perfmonc := 208
	if nmiprof.LVTmask {
		mask := uint32(1 << 16)
		lap[perfmonc] = mask
		_pmcreset(false)
	} else {
		// unmask perf LVT, reset pmc
		nmidelmode :=  uint32(0x4 << 8)
		lap[perfmonc] = nmidelmode
		_pmcreset(true)
	}
}

//go:nosplit
func _pmcreset(en bool) {
	ia32_pmc0 := 0xc1
	ia32_perfevtsel0 := 0x186
	ia32_perf_global_ovf_ctrl := 0x390
	ia32_debugctl := 0x1d9
	ia32_global_ctrl := 0x38f
	//ia32_perf_global_status := uint32(0x38e)

	if en {
		// disable perf counter before clearing
		Wrmsr(ia32_perfevtsel0, 0)

		// clear overflow
		Wrmsr(ia32_perf_global_ovf_ctrl, 1)

		r := dumrand(nmiprof.evtmin, nmiprof.evtmax)
		Wrmsr(ia32_pmc0, -int(r))

		freeze_pmc_on_pmi := 1 << 12
		Wrmsr(ia32_debugctl, freeze_pmc_on_pmi)
		// cpu clears global_ctrl on PMI if freeze-on-pmi is set.
		// re-enable
		Wrmsr(ia32_global_ctrl, 1)

		v := nmiprof.evtsel
		Wrmsr(ia32_perfevtsel0, v)
	} else {
		Wrmsr(ia32_perfevtsel0, 0)
	}

	// the write to debugctl enabling LBR must come after clearing overflow
	// via global_ovf_ctrl; otherwise the processor instantly clears lbr...
	//_lbrreset(en)
}

//go:nosplit
func sc_setup() {
	// disable interrupts
	Outb(com1 + 1, 0)

	// set divisor latch bit to set divisor bytes
	Outb(com1 + 3, 0x80)

	// set both bytes for divisor baud rate
	Outb(com1 + 0, 115200/115200)
	Outb(com1 + 1, 0)

	// 8 bit words, one stop bit, no parity
	Outb(com1 + 3, 0x03)

	// configure FIFO for interrupts: maximum FIFO size, clear
	// transmit/receive FIFOs, and enable FIFOs.
	Outb(com1 + 2, 0xc7)
	Outb(com1 + 4, 0x0b)
	Outb(com1 + 1, 1)
}

const com1 = uint16(0x3f8)

//go:nosplit
func sc_put_(c int8) {
	lstatus := uint16(5)
	for Inb(com1 + lstatus) & 0x20 == 0 {
	}
	Outb(com1, uint8(c))
}

//go:nosplit
func sc_put(c int8) {
	if c == '\n' {
		sc_put_('\r')
	}
	sc_put_(c)
	if c == '\b' {
		// clear the previous character
		sc_put_(' ')
		sc_put_('\b')
	}
}

type put_t struct {
	vx		int
	vy		int
	fakewrap	bool
}

var put put_t

//go:nosplit
func vga_put(c int8, attr int8) {
	p := (*[1999]int16)(unsafe.Pointer(uintptr(0xb8000)))
	if c != '\n' {
		// erase the previous line
		a := int16(attr) << 8
		backspace := c == '\b'
		if backspace {
			put.vx--
			c = ' '
		}
		v := a | int16(c)
		p[put.vy * 80 + put.vx] = v
		if !backspace {
			put.vx++
		}
		put.fakewrap = false
	} else {
		// if we wrapped the text because of a long line in the
		// immediately previous call to vga_put, don't add another
		// newline if we asked to print '\n'.
		if put.fakewrap {
			put.fakewrap = false
		} else {
			put.vx = 0
			put.vy++
		}
	}
	if put.vx >= 79 {
		put.vx = 0
		put.vy++
		put.fakewrap = true
	}

	if put.vy >= 25 {
		put.vy = 0
	}
	if put.vx == 0 {
		for i := 0; i < 79; i++ {
			p[put.vy * 80 + put.vx + i] = 0
		}
	}
}

var SCenable bool = true

//go:nosplit
func putch(c int8) {
	vga_put(c, 0x7)
	if SCenable {
		sc_put(c)
	}
}

//go:nosplit
func putcha(c int8, a int8) {
	vga_put(c, a)
	if SCenable {
		sc_put(c)
	}
}

//go:nosplit
func cls() {
	for i:= 0; i < 1974; i++ {
		vga_put(' ', 0x7)
	}
	sc_put('c')
	sc_put('l')
	sc_put('s')
	sc_put('\n')
}

func Trapsched() {
	mcall(trapsched_m)
}

// called only once to setup
func Trapinit() {
	mcall(trapinit_m)
}

// G_ prefix means a function had to have both C and Go versions while the
// conversion is underway. remove prefix afterwards. we need two versions of
// functions that take a string as an argument since string literals are
// different data types in C and Go.
var Halt uint32

// TEMPORARY CRAP
func _pmsg(*int8)
func invlpg(uintptr)
func rflags() uintptr

// wait until remove definition from proc.c
//type spinlock_t struct {
//	v	uint32
//}

//go:nosplit
func splock(l *spinlock_t) {
	for {
		if xchg(&l.v, 1) == 0 {
			break
		}
		for l.v != 0 {
			htpause()
		}
	}
}

//go:nosplit
func spunlock(l *spinlock_t) {
	//atomicstore(&l.v, 0)
	l.v = 0
}

// since this lock may be taken during an interrupt (only under fatal error
// conditions), interrupts must be cleared before attempting to take this lock.
var pmsglock = &spinlock_t{}

//go:nosplit
func _G_pmsg(msg string) {
	putch(' ');
	// can't use range since it results in calls stringiter2 which has the
	// stack splitting proglogue
	for i := 0; i < len(msg); i++ {
		putch(int8(msg[i]))
	}
}

// msg must be utf-8 string
//go:nosplit
func G_pmsg(msg string) {
	fl := Pushcli()
	splock(pmsglock)
	_G_pmsg(msg)
	spunlock(pmsglock)
	Popcli(fl)
}

//go:nosplit
func _pnum(n uintptr) {
	putch(' ')
	for i := 60; i >= 0; i -= 4 {
		cn := (n >> uint(i)) & 0xf
		if cn <= 9 {
			putch(int8('0' + cn))
		} else {
			putch(int8('A' + cn - 10))
		}
	}
}

//go:nosplit
func pnum(n uintptr) {
	fl := Pushcli()
	splock(pmsglock)
	_pnum(n)
	spunlock(pmsglock)
	Popcli(fl)
}

func pmsg(msg *int8)

//go:nosplit
func pancake(msg *int8, addr uintptr) {
	cli()
	atomicstore(&Halt, 1)
	_pmsg(msg)
	_pnum(addr)
	_G_pmsg("PANCAKE")
	for {
		p := (*uint16)(unsafe.Pointer(uintptr(0xb8002)))
		*p = 0x1400 | 'F'
	}
}
//go:nosplit
func G_pancake(msg string, addr uintptr) {
	cli()
	atomicstore(&Halt, 1)
	_G_pmsg(msg)
	_pnum(addr)
	_G_pmsg("PANCAKE")
	for {
		p := (*uint16)(unsafe.Pointer(uintptr(0xb8002)))
		*p = 0x1400 | 'F'
	}
}


//go:nosplit
func chkalign(_p unsafe.Pointer, n uintptr) {
	p := uintptr(_p)
	if p & (n - 1) != 0 {
		G_pancake("not aligned", p)
	}
}

//go:nosplit
func chksize(n uintptr, exp uintptr) {
	if n != exp {
		G_pancake("size mismatch", n)
	}
}

type pdesc_t struct {
	limit	uint16
	addrlow uint16
	addrmid	uint32
	addrhi	uint16
	_res1	uint16
	_res2	uint32
}

type seg64_t struct {
	lim	uint16
	baselo	uint16
	rest	uint32
}

type tss_t struct {
	_res0 uint32

	rsp0l uint32
	rsp0h uint32
	rsp1l uint32
	rsp1h uint32
	rsp2l uint32
	rsp2h uint32

	_res1 [2]uint32

	ist1l uint32
	ist1h uint32
	ist2l uint32
	ist2h uint32
	ist3l uint32
	ist3h uint32
	ist4l uint32
	ist4h uint32
	ist5l uint32
	ist5h uint32
	ist6l uint32
	ist6h uint32
	ist7l uint32
	ist7h uint32

	_res2 [2]uint32

	_res3 uint16
	iobmap uint16
	_align uint64
}

const (
	P 	uint32 = (1 << 15)
	PS	uint32 = (P | (1 << 12))
	G 	uint32 = (0 << 23)
	D 	uint32 = (1 << 22)
	L 	uint32 = (1 << 21)
	CODE	uint32 = (0x0a << 8)
	DATA	uint32 = (0x02 << 8)
	TSS	uint32 = (0x09 << 8)
	USER	uint32 = (0x60 << 8)
	INT	uint16 = (0x0e << 8)

	KCODE64		= 1
)

var _segs = [7 + 2*MAXCPUS]seg64_t{
	// 0: null segment
	{0, 0, 0},
	// 1: 64 bit kernel code
	{0, 0, PS | CODE | G | L},
	// 2: 64 bit kernel data
	{0, 0, PS | DATA | G | D},
	// 3: FS segment
	{0, 0, PS | DATA | G | D},
	// 4: GS segment. the sysexit instruction also requires that the
	// difference in indicies for the user code segment descriptor and the
	// kernel code segment descriptor is 4.
	{0, 0, PS | DATA | G | D},
	// 5: 64 bit user code
	{0, 0, PS | CODE | USER | G | L},
	// 6: 64 bit user data
	{0, 0, PS | DATA | USER | G | D},
	// 7: 64 bit TSS segment (occupies two segment descriptor entries)
	{0, 0, P | TSS | G},
	{0, 0, 0},
}

var _tss [MAXCPUS]tss_t

//go:nosplit
func tss_set(id uint, rsp, nmi uintptr) *tss_t {
	sz := unsafe.Sizeof(_tss[id])
	if sz != 104 + 8 {
		panic("bad tss_t")
	}
	p := &_tss[id]
	p.rsp0l = uint32(rsp)
	p.rsp0h = uint32(rsp >> 32)

	p.ist1l = uint32(rsp)
	p.ist1h = uint32(rsp >> 32)

	p.ist2l = uint32(nmi)
	p.ist2h = uint32(nmi >> 32)

	p.iobmap = uint16(sz)

	up := unsafe.Pointer(p)
	chkalign(up, 16)
	return p
}

// maps cpu number to the per-cpu TSS segment descriptor in the GDT
//go:nosplit
func segnum(cpunum uint) uint {
	return 7 + 2*cpunum
}

//go:nosplit
func tss_seginit(cpunum uint, _tssaddr *tss_t, lim uintptr) {
	seg := &_segs[segnum(cpunum)]
	seg.rest = P | TSS | G

	seg.lim = uint16(lim)
	seg.rest |= uint32((lim >> 16) & 0xf) << 16

	base := uintptr(unsafe.Pointer(_tssaddr))
	seg.baselo = uint16(base)
	seg.rest |= uint32(uint8(base >> 16))
	seg.rest |= uint32(uint8(base >> 24) << 24)

	seg = &_segs[segnum(cpunum) + 1]
	seg.lim = uint16(base >> 32)
	seg.baselo = uint16(base >> 48)
}

//go:nosplit
func tss_init(cpunum uint) uintptr {
	intstk := 0xa100001000 + uintptr(cpunum)*4*PGSIZE
	nmistk := 0xa100003000 + uintptr(cpunum)*4*PGSIZE
	// BSP maps AP's stack for them
	if cpunum == 0 {
		alloc_map(intstk - 1, PTE_W, true)
		alloc_map(nmistk - 1, PTE_W, true)
	}
	rsp := intstk
	rspnmi := nmistk
	tss := tss_set(cpunum, rsp, rspnmi)
	tss_seginit(cpunum, tss, unsafe.Sizeof(tss_t{}) - 1)
	segselect := segnum(cpunum) << 3
	ltr(segselect)
	cpus[lap_id()].rsp = rsp
	return rsp
}

//go:nosplit
func pdsetup(pd *pdesc_t, _addr unsafe.Pointer, lim uintptr) {
	chkalign(_addr, 8)
	addr := uintptr(_addr)
	pd.limit = uint16(lim)
	pd.addrlow = uint16(addr)
	pd.addrmid = uint32(addr >> 16)
	pd.addrhi = uint16(addr >> 48)
}

//go:nosplit
func hexdump(_p unsafe.Pointer, sz uintptr) {
	for i := uintptr(0); i < sz; i++ {
		p := (*uint8)(unsafe.Pointer(uintptr(_p) + i))
		_pnum(uintptr(*p))
	}
}

// must be nosplit since stack splitting prologue uses FS which this function
// initializes.
//go:nosplit
func seg_setup() {
	p := pdesc_t{}
	chksize(unsafe.Sizeof(seg64_t{}), 8)
	pdsetup(&p, unsafe.Pointer(&_segs[0]), unsafe.Sizeof(_segs) - 1)
	lgdt(p)

	// now that we have a GDT, setup tls for the first thread.
	// elf tls specification defines user tls at -16(%fs)
	t := uintptr(unsafe.Pointer(&tls0[0]))
	tlsaddr := int(t + 16)
	// we must set fs/gs at least once before we use the MSRs to change
	// their base address. the MSRs write directly to hidden segment
	// descriptor cache, and if we don't explicitly fill the segment
	// descriptor cache, the writes to the MSRs are thrown out (presumably
	// because the caches are thought to be invalid).
	fs_null()
	ia32_fs_base := 0xc0000100
	Wrmsr(ia32_fs_base, tlsaddr)
}

// interrupt entries, defined in runtime/asm_amd64.s
func Xdz()
func Xrz()
func Xnmi()
func Xbp()
func Xov()
func Xbnd()
func Xuo()
func Xnm()
func Xdf()
func Xrz2()
func Xtss()
func Xsnp()
func Xssf()
func Xgp()
func Xpf()
func Xrz3()
func Xmf()
func Xac()
func Xmc()
func Xfp()
func Xve()
func Xtimer()
func Xspur()
func Xyield()
func Xsyscall()
func Xtlbshoot()
func Xsigret()
func Xperfmask()
func Xirq1()
func Xirq2()
func Xirq3()
func Xirq4()
func Xirq5()
func Xirq6()
func Xirq7()
func Xirq8()
func Xirq9()
func Xirq10()
func Xirq11()
func Xirq12()
func Xirq13()
func Xirq14()
func Xirq15()

type idte_t struct {
	baselow	uint16
	segsel	uint16
	details	uint16
	basemid	uint16
	basehi	uint32
	_res	uint32
}

const idtsz uintptr = 128
var _idt [idtsz]idte_t

//go:nosplit
func int_set(idx int, intentry func(), istn int) {
	var f func()
	f = intentry
	entry := **(**uint)(unsafe.Pointer(&f))

	p := &_idt[idx]
	p.baselow = uint16(entry)
	p.basemid = uint16(entry >> 16)
	p.basehi = uint32(entry >> 32)

	p.segsel = uint16(KCODE64 << 3)

	p.details = uint16(P) | INT | uint16(istn & 0x7)
}

//go:nosplit
func int_setup() {
	chksize(unsafe.Sizeof(idte_t{}), 16)
	chksize(unsafe.Sizeof(_idt), idtsz*16)
	chkalign(unsafe.Pointer(&_idt[0]), 8)

	// cpu exceptions
	int_set(0,   Xdz,  0)
	int_set(1,   Xrz,  0)
	int_set(2,   Xnmi, 2)
	int_set(3,   Xbp,  0)
	int_set(4,   Xov,  0)
	int_set(5,   Xbnd, 0)
	int_set(6,   Xuo,  0)
	int_set(7,   Xnm,  0)
	int_set(8,   Xdf,  1)
	int_set(9,   Xrz2, 0)
	int_set(10,  Xtss, 0)
	int_set(11,  Xsnp, 0)
	int_set(12,  Xssf, 0)
	int_set(13,  Xgp,  1)
	int_set(14,  Xpf,  1)
	int_set(15,  Xrz3, 0)
	int_set(16,  Xmf,  0)
	int_set(17,  Xac,  0)
	int_set(18,  Xmc,  0)
	int_set(19,  Xfp,  0)
	int_set(20,  Xve,  0)

	// interrupts
	irqbase := 32
	int_set(irqbase+ 0,  Xtimer,  1)
	int_set(irqbase+ 1,  Xirq1,   1)
	int_set(irqbase+ 2,  Xirq2,   1)
	int_set(irqbase+ 3,  Xirq3,   1)
	int_set(irqbase+ 4,  Xirq4,   1)
	int_set(irqbase+ 5,  Xirq5,   1)
	int_set(irqbase+ 6,  Xirq6,   1)
	int_set(irqbase+ 7,  Xirq7,   1)
	int_set(irqbase+ 8,  Xirq8,   1)
	int_set(irqbase+ 9,  Xirq9,   1)
	int_set(irqbase+10,  Xirq10,  1)
	int_set(irqbase+11,  Xirq11,  1)
	int_set(irqbase+12,  Xirq12,  1)
	int_set(irqbase+13,  Xirq13,  1)
	int_set(irqbase+14,  Xirq14,  1)
	int_set(irqbase+15,  Xirq15,  1)

	int_set(48,  Xspur,    1)
	// no longer used
	//int_set(49,  Xyield,   1)
	//int_set(64,  Xsyscall, 1)

	int_set(70,  Xtlbshoot, 1)
	// no longer used
	//int_set(71,  Xsigret,   1)
	int_set(72,  Xperfmask, 1)

	p := pdesc_t{}
	pdsetup(&p, unsafe.Pointer(&_idt[0]), unsafe.Sizeof(_idt) - 1)
	lidt(p)
}

const (
	PTE_P		uintptr = 1 << 0
	PTE_W		uintptr = 1 << 1
	PTE_U		uintptr = 1 << 2
	PTE_PCD		uintptr = 1 << 4
	PGSIZE		uintptr = 1 << 12
	PGOFFMASK	uintptr = PGSIZE - 1
	PGMASK		uintptr = ^PGOFFMASK

	// special pml4 slots, agreed upon with the bootloader (which creates
	// our pmap).
	// highest runtime heap mapping
	VUEND		uintptr = 0x42
	// recursive mapping
	VREC		uintptr = 0x42
	// available mapping
	VTEMP		uintptr = 0x43
)

// physical address of kernel's pmap, given to us by bootloader
var p_kpmap uintptr

//go:nosplit
func pml4x(va uintptr) uintptr {
	return (va >> 39) & 0x1ff
}

//go:nosplit
func slotnext(va uintptr) uintptr {
	return ((va << 9) & ((1 << 48) - 1))
}

//go:nosplit
func pgroundup(va uintptr) uintptr {
	return (va + PGSIZE - 1) & PGMASK
}

//go:nosplit
func pgrounddown(va uintptr) uintptr {
	return va & PGMASK
}

//go:nosplit
func caddr(l4 uintptr, ppd uintptr, pd uintptr, pt uintptr,
    off uintptr) uintptr {
	ret := l4 << 39 | ppd << 30 | pd << 21 | pt << 12
	ret += off*8
	return uintptr(ret)
}

// XXX XXX XXX get rid of create
//go:nosplit
func pgdir_walk(_va uintptr, create bool) *uintptr {
	v := pgrounddown(_va)
	if v == 0 && create {
		G_pancake("map zero pg", _va)
	}
	slot0 := pml4x(v)
	if slot0 == VREC {
		G_pancake("map in VREC", _va)
	}
	pml4 := caddr(VREC, VREC, VREC, VREC, slot0)
	return pgdir_walk1(pml4, slotnext(v), create)
}

//go:nosplit
func pgdir_walk1(slot, van uintptr, create bool) *uintptr {
	ns := slotnext(slot)
	ns += pml4x(van)*8
	if pml4x(ns) != VREC {
		return (*uintptr)(unsafe.Pointer(slot))
	}
	sp := (*uintptr)(unsafe.Pointer(slot))
	if *sp & PTE_P == 0 {
		if !create{
			return nil
		}
		p_pg := get_pg()
		zero_phys(p_pg)
		*sp = p_pg | PTE_P | PTE_W
	}
	return pgdir_walk1(ns, slotnext(van), create)
}

//go:nosplit
func zero_phys(_phys uintptr) {
	rec := caddr(VREC, VREC, VREC, VREC, VTEMP)
	pml4 := (*uintptr)(unsafe.Pointer(rec))
	if *pml4 & PTE_P != 0 {
		G_pancake("vtemp in use", *pml4)
	}
	phys := pgrounddown(_phys)
	*pml4 = phys | PTE_P | PTE_W
	_tva := caddr(VREC, VREC, VREC, VTEMP, 0)
	tva := unsafe.Pointer(_tva)
	memclr(tva, PGSIZE)
	*pml4 = 0
	invlpg(_tva)
}

// this physical allocation code is temporary. biscuit probably shouldn't
// bother resizing its heap, ever. instead of providing a fake mmap to the
// runtime, the runtime should simply mmap its entire heap during
// initialization according to the amount of available memory.
//
// XXX when you write the new code, check and see if we can use ACPI to find
// available memory instead of e820. since e820 is only usable in realmode, we
// have to have e820 code in the bootloader. it would be better to have such
// memory management code in the kernel and not the bootloader.

type e820_t struct {
	start	uintptr
	len	uintptr
}

// "secret structure". created by bootloader for passing info to the kernel.
type secret_t struct {
	e820p	uintptr
	pmap	uintptr
	freepg	uintptr
}

// regions of memory not included in the e820 map, into which we cannot
// allocate
type badregion_t struct {
	start	uintptr
	end	uintptr
}

var badregs = []badregion_t{
	// VGA
	{0xa0000, 0x100000},
	// secret storage
	{0x7000, 0x8000},
}

//go:nosplit
func skip_bad(cur uintptr) uintptr {
	for _, br := range badregs {
		if cur >= br.start && cur < br.end {
			return br.end
		}
	}
	return cur
}

var pgfirst uintptr
var pglast uintptr

//go:nosplit
func phys_init() {
	sec := (*secret_t)(unsafe.Pointer(uintptr(0x7c00)))
	found := false
	base := sec.e820p
	// bootloader provides 15 e820 entries at most (it panicks if the PC
	// provides more).
	for i := uintptr(0); i < 15; i++ {
		ep := (*e820_t)(unsafe.Pointer(base + i*28))
		if ep.len == 0 {
			continue
		}
		endpg := ep.start + ep.len
		if pgfirst >= ep.start && pgfirst < endpg {
			pglast = endpg
			found = true
			break
		}
	}
	if !found {
		G_pancake("e820 problems", pgfirst)
	}
	if pgfirst & PGOFFMASK != 0 {
		G_pancake("pgfist not aligned", pgfirst)
	}
}

//go:nosplit
func get_pg() uintptr {
	if pglast == 0 {
		phys_init()
	}
	pgfirst = skip_bad(pgfirst)
	if pgfirst >= pglast {
		G_pancake("oom", pglast)
	}
	ret := pgfirst
	pgfirst += PGSIZE
	return ret
}

//go:nosplit
func alloc_map(va uintptr, perms uintptr, fempty bool) {
	pte := pgdir_walk(va, true)
	old := *pte
	if old & PTE_P != 0 && fempty {
		G_pancake("expected empty pte", old)
	}
	p_pg := get_pg()
	zero_phys(p_pg)
	// XXX goodbye, memory
	*pte = p_pg | perms | PTE_P
	if old & PTE_P != 0 {
		invlpg(va)
	}
}

const fxwords = 512/8
var fxinit [fxwords]uintptr

// nosplit because APs call this function before FS is setup
//go:nosplit
func fpuinit(amfirst bool) {
	finit()
	cr0 := rcr0()
	// clear EM
	cr0 &^= (1 << 2)
	// set MP
	cr0 |= 1 << 1
	lcr0(cr0);

	cr4 := rcr4()
	// set OSFXSR
	cr4 |= 1 << 9
	lcr4(cr4);

	if amfirst {
		chkalign(unsafe.Pointer(&fxinit[0]), 16)
		fxsave(&fxinit)

		// XXX XXX XXX XXX XXX XXX XXX dont forget to do this once
		// thread code is converted to go
		G_pmsg("VERIFY FX FOR THREADS\n")
	}
}

// LAPIC registers
const (
	LAPID		= 0x20/4
	LAPEOI		= 0xb0/4
	LAPVER		= 0x30/4
	LAPDCNT		= 0x3e0/4
	LAPICNT		= 0x380/4
	LAPCCNT		= 0x390/4
	LVSPUR		= 0xf0/4
	LVTIMER		= 0x320/4
	LVCMCI		= 0x2f0/4
	LVINT0		= 0x350/4
	LVINT1		= 0x360/4
	LVERROR		= 0x370/4
	LVPERF		= 0x340/4
	LVTHERMAL	= 0x330/4
)

var _lapaddr uintptr

//go:nosplit
func rlap(reg uint) uint32 {
	if _lapaddr == 0 {
		G_pancake("lapic not init", 0)
	}
	lpg := (*[PGSIZE/4]uint32)(unsafe.Pointer(_lapaddr))
	return atomicload(&lpg[reg])
}

//go:nosplit
func wlap(reg uint, val uint32) {
	if _lapaddr == 0 {
		G_pancake("lapic not init", 0)
	}
	lpg := (*[PGSIZE/4]uint32)(unsafe.Pointer(_lapaddr))
	lpg[reg] = val
}

//go:nosplit
func lap_id() uint32 {
	if rflags() & TF_FL_IF != 0 {
		G_pancake("interrupts must be cleared", 0)
	}
	if _lapaddr == 0 {
		G_pancake("lapic not init", 0)
	}
	lpg := (*[PGSIZE/4]uint32)(unsafe.Pointer(_lapaddr))
	return lpg[LAPID] >> 24
}

//go:nosplit
func lap_eoi() {
	if _lapaddr == 0 {
		G_pancake("lapic not init", 0)
	}
	wlap(LAPEOI, 0)
}

// PIT registers
const (
	CNT0	uint16 = 0x40
	CNTCTL	uint16 = 0x43
	_pitfreq	= 1193182
	_pithz		= 100
	PITDIV		= _pitfreq/_pithz
)

//go:nosplit
func pit_ticks() uint {
	// counter latch command for counter 0
	cmd := uint8(0)
	Outb(CNTCTL, cmd)
	low := Inb(CNT0)
	hi := Inb(CNT0)
	return hi << 8 | low
}

//go:nosplit
func pit_enable() {
	// rate generator mode, lsb then msb (if square wave mode is used, the
	// PIT uses div/2 for the countdown since div is taken to be the period
	// of the wave)
	Outb(CNTCTL, 0x34)
	Outb(CNT0, uint8(PITDIV & 0xff))
	Outb(CNT0, uint8(PITDIV >> 8))
}

func pit_disable() {
	// disable PIT: one-shot, lsb then msb
	Outb(CNTCTL, 0x32);
	Outb(CNT0, uint8(PITDIV & 0xff))
	Outb(CNT0, uint8(PITDIV >> 8))
}

// wait until 8254 resets the counter
//go:nosplit
func pit_phasewait() {
	// 8254 timers are 16 bits, thus always smaller than last;
	last := uint(1 << 16)
	for {
		cur := pit_ticks()
		if cur > last {
			return
		}
		last = cur
	}
}

var _lapic_quantum uint32

//go:nosplit
func lapic_setup(calibrate int32) {
	la := uintptr(0xfee00000)

	if calibrate != 0 {
		// map lapic IO mem
		pte := pgdir_walk(la, false)
		if pte != nil && *pte & PTE_P != 0 {
			G_pancake("lapic mem already mapped", 0)
		}
	}

	pte := pgdir_walk(la, true)
	*pte = la | PTE_W | PTE_P | PTE_PCD
	_lapaddr = la

	lver := rlap(LAPVER)
	if lver < 0x10 {
		G_pancake("82489dx not supported", uintptr(lver))
	}

	// enable lapic, set spurious int vector
	apicenable := 1 << 8
	wlap(LVSPUR, uint32(apicenable | TRAP_SPUR))

	// timer: periodic, int 32
	periodic := 1 << 17
	wlap(LVTIMER, uint32(periodic | TRAP_TIMER))
	// divide by 1
	divone := uint32(0xb)
	wlap(LAPDCNT, divone)

	if calibrate != 0 {
		// figure out how many lapic ticks there are in a second; first
		// setup 8254 PIT since it has a known clock frequency. openbsd
		// uses a similar technique.
		pit_enable()

		// start lapic counting
		wlap(LAPICNT, 0x80000000)
		pit_phasewait()
		lapstart := rlap(LAPCCNT)
		cycstart := Rdtsc()

		frac := 10
		// XXX only wait for 100ms instead of 1s
		for i := 0; i < _pithz/frac; i++ {
			pit_phasewait()
		}

		lapend := rlap(LAPCCNT)
		if lapend > lapstart {
			G_pancake("lapic timer wrapped?", uintptr(lapend))
		}
		lapelapsed := (lapstart - lapend)*uint32(frac)
		cycelapsed := (Rdtsc() - cycstart)*uint64(frac)
		G_pmsg("LAPIC Mhz:")
		pnum(uintptr(lapelapsed/(1000 * 1000)))
		G_pmsg("\n")
		_lapic_quantum = lapelapsed / HZ

		G_pmsg("CPU Mhz:")
		Cpumhz = uint(cycelapsed/(1000 * 1000))
		pnum(uintptr(Cpumhz))
		G_pmsg("\n")
		Pspercycle = uint(1000000000000/cycelapsed)

		pit_disable()
	}

	// initial count; the LAPIC's frequency is not the same as the CPU's
	// frequency
	wlap(LAPICNT, _lapic_quantum)

	maskint := uint32(1 << 16)
	// mask cmci, lint[01], error, perf counters, and thermal sensor
	wlap(LVCMCI,    maskint)
	// unmask LINT0 and LINT1. soon, i will use IO APIC instead.
	wlap(LVINT0,    rlap(LVINT0) &^ maskint)
	wlap(LVINT1,    rlap(LVINT1) &^ maskint)
	wlap(LVERROR,   maskint)
	wlap(LVPERF,    maskint)
	wlap(LVTHERMAL, maskint)

	ia32_apic_base := 0x1b
	reg := uintptr(Rdmsr(ia32_apic_base))
	if reg & (1 << 11) == 0 {
		G_pancake("lapic disabled?", reg)
	}
	if (reg >> 12) != 0xfee00 {
		G_pancake("weird base addr?", reg >> 12)
	}

	lreg := rlap(LVSPUR)
	if lreg & (1 << 12) != 0 {
		G_pmsg("EOI broadcast surpression\n")
	}
	if lreg & (1 << 9) != 0 {
		G_pmsg("focus processor checking\n")
	}
	if lreg & (1 << 8) == 0 {
		G_pmsg("apic disabled\n")
	}
}

var tlbshoot_wait uintptr
var tlbshoot_pg uintptr
var tlbshoot_count uintptr
var tlbshoot_pmap uintptr
var tlbshoot_gen uint64

func Tlbadmit(p_pmap, cpuwait, pg, pgcount uintptr) uint64 {
	for !casuintptr(&tlbshoot_wait, 0, cpuwait) {
		preemptok()
	}
	xchguintptr(&tlbshoot_pg, pg)
	xchguintptr(&tlbshoot_count, pgcount)
	xchguintptr(&tlbshoot_pmap, p_pmap)
	xadd64(&tlbshoot_gen, 1)
	return tlbshoot_gen
}

func Tlbwait(gen uint64) {
	for atomicloaduintptr(&tlbshoot_wait) != 0 {
		if atomicload64(&tlbshoot_gen) != gen {
			break
		}
	}
}

// must be nosplit since called at interrupt time
//go:nosplit
func tlb_shootdown() {
	lap_eoi()
	ct := Gscpu().mythread
	if ct != nil && ct.p_pmap == tlbshoot_pmap {
		// the TLB was already invalidated since trap() currently
		// switches to kernel pmap on any exception/interrupt other
		// than NMI.
		//start := tlbshoot_pg
		//end := tlbshoot_pg + tlbshoot_count * PGSIZE
		//for ; start < end; start += PGSIZE {
		//	invlpg(start)
		//}
	}
	dur := (*uint64)(unsafe.Pointer(&tlbshoot_wait))
	v := xadd64(dur, -1)
	if v < 0 {
		G_pancake("shootwait < 0", uintptr(v))
	}
	if ct != nil {
		sched_run(ct)
	} else {
		sched_halt()
	}
}

// this function checks to see if another thread is trying to preempt this
// thread (perhaps to start a GC). this is called when go code is spinning on a
// spinlock in order to avoid a deadlock where the thread that acquired the
// spinlock starts a GC and waits forever for the spinning thread. (go code
// should probably not use spinlocks. tlb shootdown code is the only code
// protected by a spinlock since the lock must both be acquired in go code and
// in interrupt context.)
//
// alternatively, we could make sure that no allocations are made while the
// spinlock is acquired.
func preemptok() {
	gp := getg()
	StackPreempt := uintptr(0xfffffffffffffade)
	if gp.stackguard0 == StackPreempt {
		G_pmsg("!")
		// call function with stack splitting prologue
		_dummy()
	}
}

var _notdeadcode uint32
func _dummy() {
	if _notdeadcode != 0 {
		_dummy()
	}
	_notdeadcode = 0
}

// cpu exception/interrupt vectors
const (
	TRAP_NMI	= 2
	TRAP_PGFAULT	= 14
	TRAP_SYSCALL	= 64
	TRAP_TIMER	= 32
	TRAP_DISK	= (32 + 14)
	TRAP_SPUR	= 48
	TRAP_YIELD	= 49
	TRAP_TLBSHOOT	= 70
	TRAP_SIGRET	= 71
	TRAP_PERFMASK	= 72
)

var threadlock = &spinlock_t{}

// maximum # of runtime "OS" threads
const maxthreads = 64
var threads [maxthreads]thread_t

// thread states
const (
	ST_INVALID	= 0
	ST_RUNNABLE	= 1
	ST_RUNNING	= 2
	ST_WAITING	= 3
	ST_SLEEPING	= 4
	ST_WILLSLEEP	= 5
)

// scheduler constants
const (
	HZ	= 100
)

//go:nosplit
func _tchk() {
	if rflags() & TF_FL_IF != 0 {
		G_pancake("must not be interruptible", 0)
	}
	if threadlock.v == 0 {
		G_pancake("must hold threadlock", 0)
	}
}

func thread_avail() int {
	_tchk()
	for i := range threads {
		if threads[i].status == ST_INVALID {
			return i
		}
	}
	G_pancake("no available threads", maxthreads)
	return -1
}

//go:nosplit
func sched_halt() {
	cpu_halt(Gscpu().rsp)
}

//go:nosplit
func sched_run(t *thread_t) {
	if t.tf[TF_RFLAGS] & TF_FL_IF == 0 {
		G_pancake("thread not interurptible", 0)
	}
	Gscpu().mythread = t
	fxrstor(&t.fx)
	trapret(&t.tf, t.p_pmap)
}

//go:nosplit
func wakeup() {
	_tchk()
	now := hack_nanotime()
	timedout := -110
	for i := range threads {
		t := &threads[i]
		sf := t.sleepfor
		if t.status == ST_SLEEPING && sf != -1 && sf < now {
			t.status = ST_RUNNABLE
			t.sleepfor = 0
			t.futaddr = 0
			t.sleepret = timedout
		}
	}
}

//go:nosplit
func yieldy() {
	_tchk()
	cpu := Gscpu()
	ct := cpu.mythread
	_ti := (uintptr(unsafe.Pointer(ct)) -
	    uintptr(unsafe.Pointer(&threads[0])))/unsafe.Sizeof(thread_t{})
	ti := int(_ti)
	start := (ti + 1) % maxthreads
	if ct == nil {
		start = 0
	}
	for i := 0; i < maxthreads; i++ {
		idx := (start + i) % maxthreads
		t := &threads[idx]
		if t.status == ST_RUNNABLE {
			t.status = ST_RUNNING
			spunlock(threadlock)
			sched_run(t)
		}
	}
	cpu.mythread = nil
	spunlock(threadlock)
	sched_halt()
}

func find_empty(sz uintptr) uintptr {
	v := caddr(0, 0, 0, 1, 0)
	cantuse := uintptr(0xf0)
	for {
		pte := pgdir_walk(v, false)
		if pte == nil || (*pte != cantuse && *pte & PTE_P == 0) {
			failed := false
			for i := uintptr(0); i < sz; i += PGSIZE {
				pte = pgdir_walk(v + i, false)
				if pte != nil &&
				    (*pte & PTE_P != 0 || *pte == cantuse) {
					failed = true
					v += i
					break
				}
			}
			if !failed {
				return v
			}
		}
		v += PGSIZE
	}
}

func prot_none(v, sz uintptr) {
	for i := uintptr(0); i < sz; i += PGSIZE {
		pte := pgdir_walk(v + i, true)
		if pte != nil {
			*pte = *pte & ^PTE_P
			invlpg(v + i)
		}
	}
}

var maplock = &spinlock_t{}

// this flag makes hack_mmap panic if a new pml4 entry is ever added to the
// kernel's pmap. we want to make sure all kernel mappings added after bootup
// fall into the same pml4 entry so that all the kernel mappings can be easily
// shared in user process pmaps.
var _nopml4 bool

func Pml4freeze() {
	_nopml4 = true
}

func hack_mmap(va, _sz uintptr, _prot uint32, _flags uint32,
    fd int32, offset int32) uintptr {
	fl := Pushcli()
	splock(maplock)

	MAP_ANON := uintptr(0x20)
	MAP_PRIVATE := uintptr(0x2)
	PROT_NONE := uintptr(0x0)
	PROT_WRITE := uintptr(0x2)

	prot := uintptr(_prot)
	flags := uintptr(_flags)
	var vaend uintptr
	var perms uintptr
	var ret uintptr
	var t uintptr
	pgleft := pglast - pgfirst
	sz := pgroundup(_sz)
	if sz > pgleft {
		ret = ^uintptr(0)
		goto out
	}
	sz = pgroundup(va + _sz)
	sz -= pgrounddown(va)
	if va == 0 {
		va = find_empty(sz)
	}
	vaend = caddr(VUEND, 0, 0, 0, 0)
	if va >= vaend || va + sz >= vaend {
		G_pancake("va space exhausted", va)
	}

	t = MAP_ANON | MAP_PRIVATE
	if flags & t != t {
		G_pancake("unexpected flags", flags)
	}
	perms = PTE_P
	if prot == PROT_NONE {
		prot_none(va, sz)
		ret = va
		goto out
	}

	if prot & PROT_WRITE != 0 {
		perms |= PTE_W
	}

	if _nopml4 {
		eidx := pml4x(va + sz - 1)
		for sidx := pml4x(va); sidx <= eidx; sidx++ {
			pml4 := caddr(VREC, VREC, VREC, VREC, sidx)
			pml4e := (*uintptr)(unsafe.Pointer(pml4))
			if *pml4e & PTE_P == 0 {
				G_pancake("new pml4 entry to kernel pmap", va)
			}
		}
	}

	for i := uintptr(0); i < sz; i += PGSIZE {
		alloc_map(va + i, perms, true)
	}
	ret = va
out:
	spunlock(maplock)
	Popcli(fl)
	return ret
}

func hack_munmap(v, _sz uintptr) {
	fl := Pushcli()
	splock(maplock)
	sz := pgroundup(_sz)
	cantuse := uintptr(0xf0)
	for i := uintptr(0); i < sz; i += PGSIZE {
		va := v + i
		pte := pgdir_walk(va, false)
		if pml4x(va) >= VUEND {
			G_pancake("high unmap", va)
		}
		// XXX goodbye, memory
		if pte != nil && *pte & PTE_P != 0 {
			// make sure these pages aren't remapped via
			// hack_munmap
			*pte = cantuse
			invlpg(va)
		}
	}
	G_pmsg("POOF\n")
	spunlock(maplock)
	Popcli(fl)
}

func clone_wrap(rip uintptr) {
	clone_call(rip)
	G_pancake("clone_wrap returned", 0)
}

func hack_clone(flags uint32, rsp uintptr, mp *m, gp *g, fn uintptr) {
	CLONE_VM := 0x100
	CLONE_FS := 0x200
	CLONE_FILES := 0x400
	CLONE_SIGHAND := 0x800
	CLONE_THREAD := 0x10000
	chk := uint32(CLONE_VM | CLONE_FS | CLONE_FILES | CLONE_SIGHAND |
	    CLONE_THREAD)
	if flags != chk {
		G_pancake("unexpected clone args", uintptr(flags))
	}
	var dur func(uintptr)
	dur = clone_wrap
	cloneaddr := **(**uintptr)(unsafe.Pointer(&dur))

	fl := Pushcli()
	splock(threadlock)

	ti := thread_avail()
	// provide fn as arg to clone_wrap
	rsp -= 8
	*(*uintptr)(unsafe.Pointer(rsp)) = fn
	rsp -= 8
	// bad return address
	*(*uintptr)(unsafe.Pointer(rsp)) = 0

	mt := &threads[ti]
	memclr(unsafe.Pointer(mt), unsafe.Sizeof(thread_t{}))
	mt.tf[TF_CS] = KCODE64 << 3
	mt.tf[TF_RSP] = rsp
	mt.tf[TF_RIP] = cloneaddr
	mt.tf[TF_RFLAGS] = rflags() | TF_FL_IF
	mt.tf[TF_FSBASE] = uintptr(unsafe.Pointer(&mp.tls[0])) + 16

	gp.m = mp
	mp.tls[0] = uintptr(unsafe.Pointer(gp))
	mp.procid = uint64(ti)
	mt.status = ST_RUNNABLE
	mt.p_pmap = p_kpmap

	mt.fx = fxinit

	spunlock(threadlock)
	Popcli(fl)
}

func hack_setitimer(timer uint32, new, old *itimerval) {
	TIMER_PROF := uint32(2)
	if timer != TIMER_PROF {
		G_pancake("weird timer", uintptr(timer))
	}

	fl := Pushcli()
	ct := Gscpu().mythread
	nsecs := new.it_interval.tv_sec * 1000000000 +
	    new.it_interval.tv_usec * 1000
	if nsecs != 0 {
		ct.prof.enabled = 1
	} else {
		ct.prof.enabled = 0
	}
	Popcli(fl)
}

func hack_sigaltstack(new, old *sigaltstackt) {
	fl := Pushcli()
	ct := Gscpu().mythread
	SS_DISABLE := int32(2)
	if new.ss_flags & SS_DISABLE != 0 {
		ct.sigstack = 0
	} else {
		ct.sigstack = uintptr(unsafe.Pointer(new.ss_sp)) +
		    uintptr(new.ss_size)
	}
	Popcli(fl)
}

func hack_write(fd int, bufn uintptr, sz uint32) int64 {
	if fd != 1 && fd != 2 {
		G_pancake("unexpected fd", uintptr(fd))
	}
	fl := Pushcli()
	splock(pmsglock)
	c := uintptr(sz)
	for i := uintptr(0); i < c; i++ {
		p := (*int8)(unsafe.Pointer(bufn + i))
		putch(*p)
	}
	spunlock(pmsglock)
	Popcli(fl)
	return int64(sz)
}

// "/etc/localtime"
var fnwhite = []int8{0x2f, 0x65, 0x74, 0x63, 0x2f, 0x6c, 0x6f, 0x63, 0x61,
    0x6c, 0x74, 0x69, 0x6d, 0x65}

// a is the C string.
func cstrmatch(a uintptr, b []int8) bool {
	for i, c := range b {
		p := (*int8)(unsafe.Pointer(a + uintptr(i)))
		if *p != c {
			return false
		}
	}
	return true
}

func hack_syscall(trap, a1, a2, a3 int64) (int64, int64, int64) {
	switch trap {
	case 1:
		r1 := hack_write(int(a1), uintptr(a2), uint32(a3))
		return r1, 0, 0
	case 2:
		enoent := int64(-2)
		if !cstrmatch(uintptr(a1), fnwhite) {
			G_pancake("unexpected open", 0)
		}
		return 0, 0, enoent
	default:
		G_pancake("unexpected syscall", uintptr(trap))
	}
	// not reached
	return 0, 0, -1
}

var futexlock = &spinlock_t{}

// XXX not sure why stack splitting prologue is not ok here
//go:nosplit
func hack_futex(uaddr *int32, op, val int32, to *timespec, uaddr2 *int32,
    val2 int32) int64 {
	stackcheck()
	FUTEX_WAIT := int32(0)
	FUTEX_WAKE := int32(1)
	uaddrn := uintptr(unsafe.Pointer(uaddr))
	ret := 0
	switch op {
	case FUTEX_WAIT:
		cli()
		splock(futexlock)
		dosleep := *uaddr == val
		if dosleep {
			ct := Gscpu().mythread
			ct.futaddr = uaddrn
			ct.status = ST_WILLSLEEP
			ct.sleepfor = -1
			if to != nil {
				t := to.tv_sec * 1000000000
				t += to.tv_nsec
				ct.sleepfor = hack_nanotime() + int(t)
			}
			mktrap(TRAP_YIELD)
			// scheduler unlocks ·futexlock and returns with
			// interrupts enabled...
			cli()
			ret = Gscpu().mythread.sleepret
			sti()
		} else {
			spunlock(futexlock)
			sti()
			eagain := -11
			ret = eagain
		}
	case FUTEX_WAKE:
		woke := 0
		cli()
		splock(futexlock)
		splock(threadlock)
		for i := 0; i < maxthreads && val > 0; i++ {
			t := &threads[i]
			st := t.status
			if t.futaddr == uaddrn && st == ST_SLEEPING {
				t.status = ST_RUNNABLE
				t.sleepfor = 0
				t.futaddr = 0
				t.sleepret = 0
				val--
				woke++
			}
		}
		spunlock(threadlock)
		spunlock(futexlock)
		sti()
		ret = woke
	default:
		G_pancake("unexpected futex op", uintptr(op))
	}
	return int64(ret)
}

func hack_usleep(delay int64) {
	ts := timespec{}
	ts.tv_sec = delay/1000000
	ts.tv_nsec = (delay%1000000)*1000
	dummy := int32(0)
	FUTEX_WAIT := int32(0)
	hack_futex(&dummy, FUTEX_WAIT, 0, &ts, nil, 0)
}

func hack_exit(code int32) {
	cli()
	Gscpu().mythread.status = ST_INVALID
	G_pmsg("exit with code")
	pnum(uintptr(code))
	G_pmsg(".\nhalting\n")
	atomicstore(&Halt, 1)
	for {
	}
}

// called in interupt context
//go:nosplit
func hack_nanotime() int {
	cyc := uint(Rdtsc())
	return int(cyc*Pspercycle/1000)
}

// XXX also called in interupt context; remove when trapstub is moved into
// runtime
//go:nosplit
func Nanotime() int {
	return hack_nanotime()
}
