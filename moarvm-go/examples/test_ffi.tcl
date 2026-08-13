# Example: Demonstrating C and Go FFI inside gperl Tcl interpreter

puts "=== Starting gperl Tcl C-FFI Demo ==="

# 1. Load Windows kernel32.dll dynamically
puts "Loading kernel32.dll via C-FFI..."
set k32 [ffi::load "kernel32.dll"]
puts "Kernel32 Handle: $k32"

# Call GetCurrentProcessId directly
set pid [ffi::call $k32 "GetCurrentProcessId" uint {}]
puts "Current Process ID from C-FFI: $pid"

# 2. Load MoarVM DLL dynamically
puts "\nLoading moar.dll via C-FFI..."
set moar [ffi::load "build/moarvm/bin/moar.dll"]
puts "MoarVM Handle: $moar"

# Call MVM_jit_support via C-FFI
set jit_support [ffi::call $moar "MVM_jit_support" int {}]
puts "MoarVM JIT Support: $jit_support (1 = Enabled)"

# Bind C functions as first-class Tcl commands
puts "\nBinding native MoarVM functions as Tcl commands..."
ffi::bind $moar "MVM_vm_create_instance" ptr {} mvm_create
ffi::bind $moar "MVM_vm_destroy_instance" void {ptr} mvm_destroy

puts "Invoking dynamically bound mvm_create command..."
set vm_ptr [mvm_create]
puts "Created MoarVM instance at pointer: $vm_ptr"

puts "Invoking dynamically bound mvm_destroy command..."
mvm_destroy $vm_ptr
puts "Destroyed MoarVM instance cleanly."

# 3. Clean up library handles
ffi::close $moar
ffi::close $k32
puts "\n=== C-FFI Demo Completed Successfully ==="
