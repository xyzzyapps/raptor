#include <stdio.h>
int main() {
    volatile long long total = 0;
    for (long long i = 1; i <= 1000000; i++) {
        total += i;
    }
    printf("%lld\n", total);
    return 0;
}
