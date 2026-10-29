#include <stdio.h>
#include <string.h>
int main() {
    char a[] = "delta";
    char b[] = "delta";
    long long c = 0;
    for (long long i = 1; i <= 1000000; i++) {
        if (strcmp(a, b) == 0) c++;
        if (strcmp(a, b) < 0) c--;
    }
    printf("%lld\n", c);
    return 0;
}
