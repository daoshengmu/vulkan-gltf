package util

import (
	"log"
	"unsafe"
	vk "github.com/vulkan-go/vulkan"
)

// A StackFrame contains all necessary information about to generate a line
// in a callstack.
// type StackFrame struct {
// 	File           string
// 	LineNumber     int
// 	Name           string
// 	Package        string
// 	ProgramCounter uintptr
// }


// func packageAndName(fn *runtime.Func) (string, string) {
// 	name := fn.Name()
// 	pkg := ""

// 	// The name includes the path name to the package, which is unnecessary
// 	// since the file name is already included.  Plus, it has center dots.
// 	// That is, we see
// 	//  runtime/debug.*T·ptrmethod
// 	// and want
// 	//  *T.ptrmethod
// 	// Since the package path might contains dots (e.g. code.google.com/...),
// 	// we first remove the path prefix if there is one.
// 	if lastslash := strings.LastIndex(name, "/"); lastslash >= 0 {
// 		pkg += name[:lastslash] + "/"
// 		name = name[lastslash+1:]
// 	}
// 	if period := strings.Index(name, "."); period >= 0 {
// 		pkg += name[:period]
// 		name = name[period+1:]
// 	}

// 	name = strings.Replace(name, "·", ".", -1)
// 	return pkg, name
// }

// // Func returns the function that this stackframe corresponds to
// func (frame *StackFrame) Func() *runtime.Func {
// 	if frame.ProgramCounter == 0 {
// 		return nil
// 	}
// 	return runtime.FuncForPC(frame.ProgramCounter)
// }

// // String returns the stackframe formatted in the same way as go does
// // in runtime/debug.Stack()
// func (frame *StackFrame) String() string {
// 	str := fmt.Sprintf("%s:%d (0x%x)\n", frame.File, frame.LineNumber, frame.ProgramCounter)

// 	source, err := frame.SourceLine()
// 	if err != nil {
// 		return str
// 	}

// 	return str + fmt.Sprintf("\t%s: %s\n", frame.Name, source)
// }

// // SourceLine gets the line of code (from File and Line) of the original source if possible
// func (frame *StackFrame) SourceLine() (string, error) {
// 	data, err := ioutil.ReadFile(frame.File)

// 	if err != nil {
// 		return "", err
// 	}

// 	lines := bytes.Split(data, []byte{'\n'})
// 	if frame.LineNumber <= 0 || frame.LineNumber >= len(lines) {
// 		return "???", nil
// 	}
// 	// -1 because line-numbers are 1 based, but our array is 0 based
// 	return string(bytes.Trim(lines[frame.LineNumber-1], " \t")), nil
// }

// // newStackFrame populates a stack frame object from the program counter.
// func newStackFrame(pc uintptr) (frame StackFrame) {

// 	frame = StackFrame{ProgramCounter: pc}
// 	if frame.Func() == nil {
// 		return
// 	}
// 	frame.Package, frame.Name = packageAndName(frame.Func())

// 	// pc -1 because the program counters we use are usually return addresses,
// 	// and we want to show the line that corresponds to the function call
// 	frame.File, frame.LineNumber = frame.Func().FileLine(pc - 1)
// 	return

// }

// func NewError(ret vk.Result) error {
// 	if ret != vk.Success {
// 		pc, _, _, ok := runtime.Caller(0)
// 		if !ok {
// 			return fmt.Errorf("vulkan error: %s (%d)",
// 				vk.Error(ret).Error(), ret)
// 		}
// 		frame := newStackFrame(pc)
// 		return fmt.Errorf("vulkan error: %s (%d) on %s",
// 			vk.Error(ret).Error(), ret, frame.String())
// 	}
// 	return nil
// }

func IsError(ret vk.Result) bool {
	return ret != vk.Success
}

func OrPanic(err error, finalizers ...func()) {
	if err != nil {
		for _, fn := range finalizers {
			fn()
		}
		panic(err)
	}
}

func Check(ret vk.Result, name string) bool {
	if err := vk.Error(ret); err != nil {
		log.Println("[WARN]", name, "failed with", err)
		return true
	}
	return false
}

func RepackUint32(data []byte) []uint32 {
	buf := make([]uint32, len(data)/4)
	vk.Memcopy(unsafe.Pointer((*sliceHeader)(unsafe.Pointer(&buf)).Data), data)
	return buf
}

type sliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}