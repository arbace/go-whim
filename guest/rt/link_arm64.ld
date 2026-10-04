/* The arm64 guest image: linked at 2 MiB, text, read-only data and data
 * in segments of their own, so that the monitor's page tables can give
 * each its permissions. */
ENTRY(_start)
PHDRS
{
    text PT_LOAD FLAGS(5);
    rodata PT_LOAD FLAGS(4);
    data PT_LOAD FLAGS(6);
}
SECTIONS
{
    . = 0x200000;
    .text : { *(.text.entry) *(.text .text.*) } :text
    . = ALIGN(4096);
    .rodata : { *(.rodata .rodata.*) } :rodata
    . = ALIGN(4096);
    .data : { *(.data .data.*) } :data
    .bss : { *(.bss .bss.*) *(COMMON) } :data
    /DISCARD/ : { *(.comment) *(.note*) *(.eh_frame*) }
}
