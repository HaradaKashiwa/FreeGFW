import { useState } from 'react'
import { IoArrowForwardSharp, IoCheckmarkCircleSharp } from 'react-icons/io5'
import { Button } from '@/components/ui/button'
import classNames from 'classnames'
import { useStartStore } from '../../store/useStartStore'
import { useGetConfigs } from '../../apis/config'

export function SelectUserType() {

    const { type, setType, setStep } = useStartStore()
    const { data: config } = useGetConfigs()

    return (
        <>
            <div className='mt-8 text-xl'>选择适合你的部署方式</div>
            <div className='grid md:grid-cols-2 gap-4 mt-4 grid-cols-1'>
                <div className={classNames({
                    'p-4 bg-white rounded-lg cursor-pointer border-2 border-stone-200 relative': true,
                    '!border-primary': type === 'newbie'
                })} onClick={() => setType('newbie')}>
                    <div>🐦 我是新手</div>
                    <div className='text-sm mt-2 opacity-70'>我第一次使用FreeGFW或第一次接触翻墙。</div>
                    {type === 'newbie' && <IoCheckmarkCircleSharp className='text-primary absolute right-3 top-3 text-2xl' />}
                </div>
                <div className={classNames({
                    'p-4 bg-white rounded-lg cursor-pointer border-2 border-stone-200 relative': true,
                    '!border-primary': type === 'expert'
                })} onClick={() => setType('expert')}>
                    <div>🧓 我是老司机</div>
                    <div className='text-sm mt-2 opacity-70'>我熟悉翻墙，知道工作原理和常见配置及术语。</div>
                    {type === 'expert' && <IoCheckmarkCircleSharp className='text-primary absolute right-3 top-3 text-2xl' />}
                </div>
            </div>
            <div className='mt-8 flex justify-between'>
                <div>
                    {!config?.ssl && <Button onClick={() => setStep('letsencrypt')} variant='outline'>
                        返回申请证书
                    </Button>}
                </div>
                <Button className='cursor-pointer' onClick={() => setStep(`for-${type}`)}>下一步 <IoArrowForwardSharp /></Button>
            </div>
        </>
    )
}