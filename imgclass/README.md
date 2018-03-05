# imgclass

Image classification tool for Classificationbox.

* Read the blog post: [Build a machine learning image classifier from photos on your hard drive very quickly](https://blog.machinebox.io/how-anyone-can-build-a-machine-learning-image-classifier-from-photos-on-your-hard-drive-very-5c20c6f2764f)

## Usage

1. Prepare teaching images
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
imgclass -teachratio 0.8 -src ./teaching-images
```

The tool will post a random 80% (`-teachratio 0.8`) of the images to Classificationbox for teaching, and the
remaining images will be used to test the model.

### Watch the magic happen

You will be prompted a few times as the tool goes through its various stages. The tool will:

1. Create a new model
1. Use a percentage of the data to teach the model
1. Use the remaining images to validate the model
1. Display the results, including the percentage accurary of the model
