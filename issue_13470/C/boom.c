// Discovered by Ian Lance Taylor (https://github.com/ianlancetaylor).
// Invoke via `gcc -static -pthread -o cboom boom.c && ./cboom`.
#include <stdio.h>
#include <ctype.h>
#include <sys/types.h>
#include <pwd.h>
#include <pthread.h>

static pthread_mutex_t mutex = PTHREAD_MUTEX_INITIALIZER;

static void *thread(void *arg) {
    struct passwd pwd;
    char buf[1024];
    struct passwd *result;
    pthread_mutex_lock(&mutex);
    getpwuid_r(0, &pwd, buf, sizeof buf, &result);
    return NULL;
}

int main() {
    pthread_t tid;
    struct passwd pwd;
    char buf[1024];
    struct passwd *result;
    void *retval;
    pthread_mutex_lock(&mutex);
    pthread_create(&tid, NULL, thread, NULL);
    getpwuid_r(0, &pwd, buf, sizeof buf, &result);
    pthread_mutex_unlock(&mutex);
    pthread_join(tid, &retval);
    return 0;
}
