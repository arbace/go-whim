package whim.rt;

/**
 * What every class a C struct or union becomes can do that the runtime's
 * functions of bytes need of it: set(o), C's assignment -- a copy of every
 * member -- and zero(), memset(p, 0, sizeof *p).  Both return the object.
 */
public interface Struct<T> {
    T set(T o);

    T zero();
}
