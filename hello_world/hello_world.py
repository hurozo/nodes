from hurozo import RemoteAgent

def my_remote_agent(name):
    outputs = {
        'greeting': f'Gwuaaak {name}',
        'shout': f'GWUAAAAK {name.upper()}'
    }
    return outputs

def main():
    RemoteAgent(my_amazing_node, {
        'inputs': ['name'],
        'outputs': ['greeting', 'shout']
    })

if __name__ == '__main__':
    main()
