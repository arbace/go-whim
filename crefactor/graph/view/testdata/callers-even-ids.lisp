;; 1 call of even in 1 function
(callers #55 even
  (in #43 odd
    #47(return #48(? #49(== #50:n@46 0) 0 #51(call #52:even@39 #53(- #54:n@46 1))))
    (in @55 even
      #59(return #60(? #61(== #62:n@58 0) 1 #63(call #64:odd@43 #65(- #66:n@58 1)))))
    (in #120 main
      #145(return
        #146(+
          #147(. #148:b@123 #149:b_ml@3)
          #150(cast int #151(sizeof #152(. #153:b@123 #154:b_ml@3)))
          #155(call #156:odd@43 #157(deref #158:p@131))
          #159(. #160:o@125 #161:b_ml@11)
          #162(-> #163(call #164:mk@102) #165:b_ml@3)
          #166(call #167(index #168:table@112 0) 2))))))
