# Example: Using MoarVM from Tcl inside gperl

puts "=== Starting gperl Tcl & MoarVM Bridge Demo ==="

# Define a pure Tcl helper proc
proc calculate_load {items factor} {
    set count [llength $items]
    return [expr $count * $factor]
}

set modules [list core parser vm bridge]
set load [calculate_load $modules 10]
puts "Calculated runtime load units: $load"

# Interact with MoarVM engine
puts "Initial MoarVM State: [moar::state]"
puts "Initializing MoarVM instance..."
set init_status [moar::init]
puts "MoarVM Init Result: $init_status"
puts "Current MoarVM State: [moar::state]"

# Configure runtime properties
moar::set_prog_name "gperl_tcl_demo"
moar::set_args {--verbose --sample-mode}
puts "Configured MoarVM program name and arguments."

# Teardown MoarVM instance
puts "Destroying MoarVM instance..."
set destroy_status [moar::destroy]
puts "MoarVM Destroy Result: $destroy_status"
puts "Final MoarVM State: [moar::state]"

puts "=== Demo Finished Successfully ==="
