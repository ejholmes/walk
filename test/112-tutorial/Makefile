OBJ = main.o parse.o
prog: $(OBJ)
	  $(CC) -o $@ $(OBJ)
main.o: main.c parse.h
	  $(CC) -c main.c
parse.o: parse.c parse.h
	  $(CC) -c parse.c
