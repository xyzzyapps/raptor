# ==============================================================================
# Raylib 5.5 Native FFI Bindings for Raku5
# ==============================================================================

# Core C Structs
struct Color {
    uint8 $r;
    uint8 $g;
    uint8 $b;
    uint8 $a;
}

struct Vector2 {
    num32 $x;
    num32 $y;
}

struct Vector3 {
    num32 $x;
    num32 $y;
    num32 $z;
}

struct Rectangle {
    num32 $x;
    num32 $y;
    num32 $width;
    num32 $height;
}

# --- Core Window & Context ---
sub InitWindow(int32 $width, int32 $height, Str $title) returns void is native('libraylib.dll') { * }
sub WindowShouldClose() returns bool is native('libraylib.dll') { * }
sub CloseWindow() returns void is native('libraylib.dll') { * }
sub IsWindowReady() returns bool is native('libraylib.dll') { * }
sub SetTargetFPS(int32 $fps) returns void is native('libraylib.dll') { * }
sub GetFPS() returns int32 is native('libraylib.dll') { * }
sub GetFrameTime() returns num32 is native('libraylib.dll') { * }
sub GetTime() returns num64 is native('libraylib.dll') { * }
sub GetScreenWidth() returns int32 is native('libraylib.dll') { * }
sub GetScreenHeight() returns int32 is native('libraylib.dll') { * }

# --- Drawing & Frame Lifecycle ---
sub BeginDrawing() returns void is native('libraylib.dll') { * }
sub EndDrawing() returns void is native('libraylib.dll') { * }
sub ClearBackground(Color $color) returns void is native('libraylib.dll') { * }

# --- 2D Shapes Drawing ---
sub DrawPixel(int32 $posX, int32 $posY, Color $color) returns void is native('libraylib.dll') { * }
sub DrawLine(int32 $startPosX, int32 $startPosY, int32 $endPosX, int32 $endPosY, Color $color) returns void is native('libraylib.dll') { * }
sub DrawCircle(int32 $centerX, int32 $centerY, num32 $radius, Color $color) returns void is native('libraylib.dll') { * }
sub DrawCircleLines(int32 $centerX, int32 $centerY, num32 $radius, Color $color) returns void is native('libraylib.dll') { * }
sub DrawRectangle(int32 $posX, int32 $posY, int32 $width, int32 $height, Color $color) returns void is native('libraylib.dll') { * }
sub DrawRectangleLines(int32 $posX, int32 $posY, int32 $width, int32 $height, Color $color) returns void is native('libraylib.dll') { * }

# --- Text & GUI Drawing ---
sub DrawFPS(int32 $posX, int32 $posY) returns void is native('libraylib.dll') { * }
sub DrawText(Str $text, int32 $posX, int32 $posY, int32 $fontSize, Color $color) returns void is native('libraylib.dll') { * }

# --- Input Handling (Keyboard & Mouse) ---
sub IsKeyPressed(int32 $key) returns bool is native('libraylib.dll') { * }
sub IsKeyDown(int32 $key) returns bool is native('libraylib.dll') { * }
sub IsKeyReleased(int32 $key) returns bool is native('libraylib.dll') { * }
sub IsMouseButtonPressed(int32 $button) returns bool is native('libraylib.dll') { * }
sub IsMouseButtonDown(int32 $button) returns bool is native('libraylib.dll') { * }
sub GetMouseX() returns int32 is native('libraylib.dll') { * }
sub GetMouseY() returns int32 is native('libraylib.dll') { * }

# Helper to construct Color instances easily
sub make_color(uint8 $r, uint8 $g, uint8 $b, uint8 $a) {
    my $c = Color.new();
    $c.r = $r;
    $c.g = $g;
    $c.b = $b;
    $c.a = $a;
    return $c;
}
