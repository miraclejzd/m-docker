package nsenter

/*
#cgo CFLAGS: -Wno-all
__attribute__((constructor)) void init(void) {
	nsenter();
}
*/
import "C"
