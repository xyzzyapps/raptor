#include <stdio.h>
#include <stdlib.h>
#include <string.h>
int main() {
    size_t cap = 64;
    size_t len = 0;
    char *s = malloc(cap);
    for (int i = 1; i <= 50000; i++) {
        if (len + 2 >= cap) {
            cap *= 2;
            s = realloc(s, cap);
        }
        s[len++] = 'x';
        s[len] = '\0';
    }
    printf("%zu\n", len);
    free(s);
    return 0;
}
