# textclass

Text classification tool for Classificationbox.

## Usage

1. Prepare teaching data
1. Run Classificationbox
1. Teach and test

### Prepare teaching data

Create a directory structure that organizes the files into classes, with each folder as the class name:

```
/teaching-items
	/class1
		class1example1.txt
		class1example2.txt
		class1example3.txt
	/class2
		class2example1.txt
		class2example2.txt
		class2example3.txt
	/class3
		class3example1.txt
		class3example2.txt
		class3example3.txt
```

The files can be text of any size, one file per example.

### Run Classificationbox

In a terminal do:

```
docker run -p 8080:8080 -e "MB_KEY=$MB_KEY" machinebox/classificationbox
```

* Get yourself an `MB_KEY` from https://machinebox.io/account 

### Teach and test

Use the `textclass` tool to teach the 

```
textclass -teachratio 0.8 -src ./teaching-items
```

The tool will post a random 80% (`-teachratio 0.8`) of the files to Classificationbox for teaching, and the
remaining items will be used to test the model.

### Watch the magic happen

You will be prompted a few times as the tool goes through its various stages. The tool will:

1. Create a new model
1. Use a percentage of the data to teach the model
1. Use the remaining items to validate the model
1. Display the results, including the percentage accurary of the model
