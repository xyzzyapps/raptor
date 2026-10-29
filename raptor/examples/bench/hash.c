#include <stdio.h>
int main() {
    long long counts[1000] = {0};
    for (long long i = 1; i <= 100000; i++) {
        counts[i % 1000]++;
    }
    long long nonzeros = 0;
    for (int i = 0; i < 1000; i++) {
        if (counts[i]) nonzeros++;
    }
    printf("%lld %lld\n", nonzeros, counts[0]);
    return 0;
}
