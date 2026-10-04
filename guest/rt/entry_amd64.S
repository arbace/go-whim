/* The amd64 guest's entry and exception vectors.  The monitor starts the
 * vCPU in 64-bit mode at _start with RSP the stack's top, RDI argc, RSI
 * argv and RDX the heap; every vector runs on the TSS's IST1 stack, so a
 * fault in the stack's guard page is reported too. */
    .section .text.entry, "ax"
    .globl _start
_start:
    xorl %ebp, %ebp
    call whim_main
1:  hlt
    jmp 1b

    .text
    .balign 16
    .globl whim_vectors
whim_vectors:
    .irp n, 0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31
    .balign 16
    .if \n == 8 || \n == 10 || \n == 11 || \n == 12 || \n == 13 || \n == 14 || \n == 17 || \n == 21 || \n == 29 || \n == 30
    .else
    pushq $0
    .endif
    pushq $\n
    jmp whim_vector_common
    .endr

whim_vector_common:
    popq %rdi           /* the vector */
    popq %rsi           /* the error code */
    movq (%rsp), %rdx   /* RIP */
    movq %cr2, %rcx     /* the faulting address */
    andq $-16, %rsp
    call whim_fault
2:  hlt
    jmp 2b
