#include <stdio.h>
#include <stdlib.h>
static int cmp(const void *a, const void *b) {
    long long diff = *(const long long*)a - *(const long long*)b;
    return (diff > 0) - (diff < 0);
}
int main() {
    long long *nums = malloc(50000 * sizeof(long long));
    for (long long i = 1; i <= 50000; i++) {
        nums[i - 1] = (i * 2654435761ULL) % 100000;
    }
    qsort(nums, 50000, sizeof(long long), cmp);
    printf("%lld %lld 50000\n", nums[0], nums[49999]);
    free(nums);
    return 0;
}
