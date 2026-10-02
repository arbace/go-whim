/* the boot files main.c starts Chez on, linked in (vfasl, 16-aligned) */
    .section .rodata
    .balign 16
    .globl whimsical_petite, whimsical_petite_end, whimsical_app, whimsical_app_end
whimsical_petite:
    .incbin "petite-v.boot"
whimsical_petite_end:
    .balign 16
whimsical_app:
    .incbin "whimsical-v.boot"
whimsical_app_end:
    .section .note.GNU-stack,"",@progbits
