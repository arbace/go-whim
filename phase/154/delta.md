154 declares nothing: the call it removes never cleared t_Co; its one effect
was a write through NULL, which no recorded case reaches.  The fix of vim's
intent -- clearing t_Co when there is no t_AB and no t_Sb -- would change
behaviour and is not this phase.
