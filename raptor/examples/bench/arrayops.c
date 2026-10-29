#include <stdio.h>
int main() {
    long long sum = 0;
    for (long long i = 1; i <= 200000; i++) {
        if (i % 3 == 0) {
            sum += (i + 1);
        }
    }
    printf("%lld\n", sum);
    return 0;
}
