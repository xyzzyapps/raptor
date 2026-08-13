# Raylib Raku5 FFI Binding Test

struct Color {
    uint8 $r;
    uint8 $g;
    uint8 $b;
    uint8 $a;
}

sub InitWindow(int32 $width, int32 $height, Str $title) returns void is native('libraylib.dll') { * }
sub WindowShouldClose() returns bool is native('libraylib.dll') { * }
sub CloseWindow() returns void is native('libraylib.dll') { * }
sub SetTargetFPS(int32 $fps) returns void is native('libraylib.dll') { * }
sub GetScreenWidth() returns int32 is native('libraylib.dll') { * }
sub GetScreenHeight() returns int32 is native('libraylib.dll') { * }

say "Initializing Raylib Window...";
InitWindow(640, 480, "Raku5 + Raylib NativeCall FFI");
SetTargetFPS(60);

my $w = GetScreenWidth();
my $h = GetScreenHeight();
say "Window Initialized! Screen Width = $w, Screen Height = $h";

# Close immediately for automated test
CloseWindow();
say "Raylib Closed cleanly!";
