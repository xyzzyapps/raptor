# ==============================================================================
# Procedural Socket Module for Raptor (TCP & UDP)
# Strictly Procedural / Functional (No OO classes)
# ==============================================================================

# TCP Functions
sub tcp_open_server(Str $port_or_addr) {
    return tcp_listen($port_or_addr);
}

sub tcp_accept_client(Str $listener_handle, Int $timeout_ms = 0) {
    return tcp_accept($listener_handle, $timeout_ms);
}

sub tcp_dial(Str $host, Str $port, Int $timeout_ms = 10000) {
    return tcp_connect($host, $port, $timeout_ms);
}

sub tcp_write(Str $sock, Str $data) {
    return tcp_send($sock, $data);
}

sub tcp_read(Str $sock, Int $max_bytes = 4096) {
    return tcp_recv($sock, $max_bytes);
}

sub tcp_shutdown(Str $handle) {
    return tcp_close($handle);
}

# UDP Functions
sub udp_open(Str $port_or_addr) {
    return udp_bind($port_or_addr);
}

sub udp_send_msg(Str $sock, Str $host, Int $port, Str $data) {
    return udp_sendto($sock, $host, $port, $data);
}

sub udp_recv_msg(Str $sock, Int $max_bytes = 4096) {
    return udp_recvfrom($sock, $max_bytes);
}

sub udp_shutdown(Str $sock) {
    return udp_close($sock);
}
