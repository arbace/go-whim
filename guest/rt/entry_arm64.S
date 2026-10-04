/* The arm64 guest's entry and exception vectors.  The monitor starts the
 * vCPU at EL1t (interrupts masked) at _start, with SP_EL0 the stack's top,
 * X0 argc, X1 argv and X2 the heap, and the MMU on.  An exception is taken
 * at EL1h, on SP_EL1, the fault stack: a fault in the stack's guard page is
 * reported too. */
    .section .text.entry, "ax"
    .globl _start
_start:
    mov x29, #0
    mov x30, #0
    bl whim_main
1:  wfi
    b 1b

    .text
    .balign 2048
    .globl whim_vectors
whim_vectors:
    .irp n, 0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15
    .balign 128
    mov x0, #\n
    mrs x1, esr_el1
    mrs x2, elr_el1
    mrs x3, far_el1
    b whim_fault
    .endr
