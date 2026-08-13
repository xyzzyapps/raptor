# Tcl on MoarVM Demo

set total 0
for {set i 1} {$i <= 10} {incr i} {
    set total [expr $total + $i]
}
puts "Sum of 1..10 = $total"

set fruit_list [list "Apple" "Banana" "Cherry"]
puts "List count: [llength $fruit_list]"
puts "First item: [lindex $fruit_list 0]"
