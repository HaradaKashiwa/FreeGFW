import { SelectUserType } from './start/select-user-type'
import { useStartStore } from '../store/useStartStore'
import { ForNewbie } from './start/for-newbie'
import { ForExpert } from './start/for-expert'
import { useGetConfigs } from '../apis/config'
import { Navigate } from 'react-router-dom'
import { useEffect } from 'react'
import { LetsEncrypt } from './start/letsencrypt'
import { Button } from '@/components/ui/button'

export default function Start() {
    const { step, setStep } = useStartStore()
    const { data: config } = useGetConfigs()
    useEffect(() => {
        if (!config?.ssl) {
            setStep('letsencrypt')
            return
        }
        setStep('select-user-type')
    }, [])
    const steps = {
        'letsencrypt': <LetsEncrypt />,
        'select-user-type': <SelectUserType />,
        'for-newbie': <ForNewbie />,
        'for-expert': <ForExpert />,
    }
    if (config?.inited) {
        return <Navigate to='/' />
    }
    return (
        <div className='max-w-2xl mx-auto px-2'>
            <div className='mt-8 text-2xl'>👋 第一次来吗？</div>
            <div className='text-sm mt-2 opacity-70'>从这里开始部署你的FreeGFW</div>

            {steps[step]}
        </div>
    )
}
