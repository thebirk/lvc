#include <stdio.h>

// Print a greeting for [name]
void sayHello(const char *name) {
	printf("Hello, %s!\n");
}

// This is the program entry point
int main(int argc, char **argv) {
	sayHello("World");
	return 0;
} 
