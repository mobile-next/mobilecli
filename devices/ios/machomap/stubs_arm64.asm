// In-process loop stubs for the signing-free mapper.
// Each is called ONCE via the debugger instead of one vCont round-trip per element.
// Uniform frame: save x19-x24, x29/x30 (64 bytes).

.text
.align 2

// rstub_dlsym(x0=fn_dlsym, x1=count, x2=names[], x3=out[], x4=handle)
//   for i in 0..count: out[i] = dlsym(handle, names[i])
.globl _rstub_dlsym
_rstub_dlsym:
    stp x19, x20, [sp, #-64]!
    stp x21, x22, [sp, #16]
    stp x23, x24, [sp, #32]
    stp x29, x30, [sp, #48]
    mov x19, x0            // fn
    mov x20, x1            // count
    mov x21, x2            // names
    mov x22, x3            // out
    mov x23, x4            // handle
    mov x24, #0            // i
1:
    cmp x24, x20
    b.ge 2f
    mov x0, x23
    ldr x1, [x21, x24, lsl #3]
    blr x19
    str x0, [x22, x24, lsl #3]
    add x24, x24, #1
    b 1b
2:
    ldp x29, x30, [sp, #48]
    ldp x23, x24, [sp, #32]
    ldp x21, x22, [sp, #16]
    ldp x19, x20, [sp], #64
    ret

// rstub_selref(x0=fn_sel, x1=count, x2=slots)
//   for i: slots[i] = sel_registerName(slots[i])   // slot holds the name ptr, replaced by SEL
.globl _rstub_selref
_rstub_selref:
    stp x19, x20, [sp, #-64]!
    stp x21, x22, [sp, #16]
    stp x23, x24, [sp, #32]
    stp x29, x30, [sp, #48]
    mov x19, x0            // fn
    mov x20, x1            // count
    mov x21, x2            // slots
    mov x24, #0            // i
1:
    cmp x24, x20
    b.ge 2f
    ldr x0, [x21, x24, lsl #3]
    blr x19
    str x0, [x21, x24, lsl #3]
    add x24, x24, #1
    b 1b
2:
    ldp x29, x30, [sp, #48]
    ldp x23, x24, [sp, #32]
    ldp x21, x22, [sp, #16]
    ldp x19, x20, [sp], #64
    ret

// rstub_class(x0=fn_readpair, x1=count, x2=classlist, x3=imageinfo)
//   for i: objc_readClassPair(classlist[i], imageinfo)
.globl _rstub_class
_rstub_class:
    stp x19, x20, [sp, #-64]!
    stp x21, x22, [sp, #16]
    stp x23, x24, [sp, #32]
    stp x29, x30, [sp, #48]
    mov x19, x0            // fn
    mov x20, x1            // count
    mov x21, x2            // classlist
    mov x22, x3            // imageinfo
    mov x24, #0            // i
1:
    cmp x24, x20
    b.ge 2f
    ldr x0, [x21, x24, lsl #3]
    mov x1, x22
    blr x19
    add x24, x24, #1
    b 1b
2:
    ldp x29, x30, [sp, #48]
    ldp x23, x24, [sp, #32]
    ldp x21, x22, [sp, #16]
    ldp x19, x20, [sp], #64
    ret
