#include <errno.h>
#include <string.h>
void geterrstr() {
    printf("%s\n", strerror(errno));
}
