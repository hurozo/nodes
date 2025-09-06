from hurozo import Node

def my_amazing_node(name):
    outputs = {
        'greeting': f'Gwuaaak {name}',
        'shout': f'GWUAAAAK {name.upper()}'
    }
    return outputs

def main():
    Node(my_amazing_node, {
        'inputs': ['name'],
        'outputs': ['greeting', 'shout']
    })

if __name__ == '__main__':
    main()
