# imgclass

Image classification tool for Classificationbox.

## Usage

1. Prepare traching images
1. Run Classificationbox
1. Teach and test

### Prepare teaching images

Create a directory structure that organizes the images into classes, with each folder as the class name:

```
/teaching-images
	/class1
		class1example1.jpg
		class1example2.jpg
		class1example3.jpg
	/class2
		class2example1.jpg
		class2example2.jpg
		class2example3.jpg
	/class3
		class3example1.jpg
		class3example2.jpg
		class3example3.jpg
```

### Run Classificationbox

In a terminal do:

```
docker run -p 8080:8080 -e "MB_KEY=$MB_KEY" machinebox/classificationbox
```

* Get yourself an `MB_KEY` from https://machinebox.io/account 

### Teach and test

Use the `imgclass` tool to teach the 

```
imgclass -testratio=0.2 ./teaching-images
```

The tool will post a random 80% of the images to Classificationbox for teaching, and the
remaining images (20% because of the `-testratio=0.2` flag) will be used to test the model.
