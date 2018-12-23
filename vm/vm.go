/*
MIT License

Copyright (c) 2018-2019 Alexandru-Paul Copil

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package vm

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/thee-engineer/mu0/module"
	"github.com/thee-engineer/mu0/mu0"
)

// VM attempts to simulate the components found on the UoM MU0 boards
type VM struct {
	isRunning  bool // State of the VM (running)
	isSleeping bool // State of the VM (sleeping)
	isInBreak  bool // State of the VM (break, wait for input)

	ACC      mu0.Word         // Accumulator (main register)
	PC       mu0.Word         // Program Counter
	Memory   [0xFFFF]mu0.Word // Physical memory space
	StopCode mu0.Word         // Exit code / Stop code

	modules []module.Module // List of peripheral devices
}

// New create a virtual machine
func New() *VM {
	return new(VM)
}

// HandleModules starts a new thread that manages all external devices (modules)
func (v *VM) HandleModules() {
	// Background task
	for {
		// Iterate modules
		for _, mod := range v.modules {
			// Skip busy modules
			if mod.IsBusy() {
				continue
			}

			//Handle module on a new thread
			go mod.Handle(&v.Memory)
		}
	}
}

// Load a compiled program into memory
func (v *VM) Load(data []byte, start int) {
	for index := start; index < len(data) && index/2 < cap(v.Memory); index += 2 {
		v.Memory[index/2] = mu0.Word(data[index])<<8 | mu0.Word(data[index+1])
	}
}

// LoadFile takes a file path and loads all binary data from it
func (v *VM) LoadFile(filePath string) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	v.Load(data, 0)
}

// Stop VM execution
func (v *VM) Stop(code mu0.Word) {
	v.StopCode = code
	v.isRunning = false
}

// AddModule appends a module to the VM, only if it's NOT running
func (v *VM) AddModule(m module.Module) error {
	// Don't append module when running
	if v.isRunning {
		return errors.New("VM already running, can't add module")
	}

	// Append module
	v.modules = append(v.modules, m)

	// No error
	return nil
}

// Run starts OP execution from the ORIG address
func (v *VM) Run() {
	v.isRunning = true

	var instruction mu0.Word // Current instruction
	var opc mu0.Word         // Operation code
	var arg mu0.Word         // Operation arg

	// Start module handler thread
	go v.HandleModules()

	for v.isRunning {
		// Check PC in memory range
		if int(v.PC) > len(v.Memory)-1 {
			log.Println("VM: PC out of memory address space")
			v.Stop(400)
			return
		}

		instruction = v.Memory[v.PC]    // Load instruction from memory (PC)
		opc = instruction & mu0.OpcMask // Extract operation code
		arg = instruction & mu0.ArgMask // Extract operation arg

		v.PC++ // Increment PC

		// Check which instruction to execute and how
		switch opc {
		case mu0.OpLDA:
			v.ACC = v.Memory[arg]
			break
		case mu0.OpSTA:
			v.Memory[arg] = v.ACC
			break
		case mu0.OpADD:
			v.ACC += v.Memory[arg]
			break
		case mu0.OpSUB:
			v.ACC -= v.Memory[arg]
			break
		case mu0.OpJMP:
			v.PC = arg
			break
		case mu0.OpJGE:
			if v.ACC >= 0 {
				v.PC = arg
			}
			break
		case mu0.OpJNE:
			if v.ACC != 0 {
				v.PC = arg
			}
			break
		case mu0.OpSTP:
			v.Stop(arg)
			break
		case mu0.OpSLP:
			// Convert argument word to duration string then duration
			d, err := time.ParseDuration(fmt.Sprintf("%dms", arg))
			if err != nil {
				log.Fatalln(err)
			}

			// Set sleeping state and sleep
			v.isSleeping = true
			time.Sleep(d)
			v.isSleeping = false

			break
		default:
			log.Fatalf("%04x %04x\n", opc, arg)
		}
	}
}

// MemoryDump writes the memory contents to stdout. It takes the max address
// to print until, if <= 0, set to max address space.
func (v *VM) MemoryDump(to int) {
	if to <= 0 {
		to = cap(v.Memory)
	}

	for index := 0; index+8 < to; index += 8 {
		fmt.Printf(
			"%04x : %04x %04x %04x %04x %04x %04x %04x %04x\n", index,
			v.Memory[index],
			v.Memory[index+1],
			v.Memory[index+2],
			v.Memory[index+3],
			v.Memory[index+4],
			v.Memory[index+5],
			v.Memory[index+6],
			v.Memory[index+7])
	}
}
